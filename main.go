package main

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// ── Model ─────────────────────────────────────────────────────────────────────

// User is our data model. Struct tags serve three purposes:
//   json:"..."      — field name when encoding/decoding JSON
//   validate:"..."  — rules run by go-playground/validator
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"  validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age"   validate:"gte=0,lte=130"`
}

// ── Validator ─────────────────────────────────────────────────────────────────

// CustomValidator wraps go-playground/validator.
// Echo's Validator interface requires exactly one method: Validate(i interface{}) error.
// We register it on Echo via e.Validator = &CustomValidator{...}, after which
// every call to c.Validate(&someStruct) routes through here.
type CustomValidator struct {
	v *validator.Validate
}

// Validate returns the raw validator.ValidationErrors so callers can inspect
// individual field failures. Returning it unwrapped lets handlers decide the
// response shape themselves — cleaner than hard-coding it here.
func (cv *CustomValidator) Validate(i any) error {
	return cv.v.Struct(i)
}

// validationErrors converts validator.ValidationErrors into a map[field]→rule
// and writes a 400 JSON response. Returns the result of c.JSON so the handler
// can do `return validationErrors(c, err)` in one line.
func validationErrors(c *echo.Context, err error) error {
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	}
	fields := make(map[string]string, len(errs))
	for _, fe := range errs {
		// fe.Field() → struct field name (e.g. "Name")
		// fe.Tag()   → the failing rule   (e.g. "required", "email", "min")
		fields[strings.ToLower(fe.Field())] = fe.Tag()
	}
	return c.JSON(http.StatusBadRequest, map[string]any{"errors": fields})
}

// ── Store ─────────────────────────────────────────────────────────────────────

var (
	store = map[string]User{}
	mu    sync.RWMutex
)

// ── Handlers ──────────────────────────────────────────────────────────────────

// GET /api/v1/users?search=<name>
func getUsers(c *echo.Context) error {
	search := c.QueryParam("search")

	mu.RLock()
	defer mu.RUnlock()

	result := []User{}
	for _, u := range store {
		if search == "" || strings.Contains(strings.ToLower(u.Name), strings.ToLower(search)) {
			result = append(result, u)
		}
	}
	return c.JSON(http.StatusOK, result)
}

// GET /api/v1/users/:id
func getUserByID(c *echo.Context) error {
	id := c.Param("id")

	mu.RLock()
	u, ok := store[id]
	mu.RUnlock()

	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.JSON(http.StatusOK, u)
}

// POST /api/v1/users
func createUser(c *echo.Context) error {
	var u User

	// Bind reads the JSON body into u. It also handles path params and query
	// params, but for POST we only care about the body here.
	if err := c.Bind(&u); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Validate runs our CustomValidator, which calls validator.Struct(u).
	// If any validate tag fails, we convert the errors to a field map and
	// return a 400 with {"errors": {"name": "min", "email": "required"}, ...}
	if err := c.Validate(&u); err != nil {
		return validationErrors(c, err)
	}

	u.ID = uuid.NewString()

	mu.Lock()
	store[u.ID] = u
	mu.Unlock()

	return c.JSON(http.StatusCreated, u)
}

// PUT /api/v1/users/:id
func updateUser(c *echo.Context) error {
	id := c.Param("id")

	mu.RLock()
	existing, ok := store[id]
	mu.RUnlock()

	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	// Initialise updated with the existing record. This means any field the
	// client omits from the request body keeps its current value (patch semantics).
	updated := existing
	if err := c.Bind(&updated); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&updated); err != nil {
		return validationErrors(c, err)
	}

	updated.ID = id // stop the client from changing the ID via the body

	mu.Lock()
	store[id] = updated
	mu.Unlock()

	return c.JSON(http.StatusOK, updated)
}

// DELETE /api/v1/users/:id
func deleteUser(c *echo.Context) error {
	id := c.Param("id")

	mu.Lock()
	_, ok := store[id]
	if ok {
		delete(store, id)
	}
	mu.Unlock()

	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.NoContent(http.StatusNoContent)
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	e := echo.New()

	// Register our validator. After this, c.Validate(&x) calls CustomValidator.Validate(x).
	e.Validator = &CustomValidator{v: validator.New()}

	g := e.Group("/api/v1")
	g.GET("/users", getUsers)
	g.GET("/users/:id", getUserByID)
	g.POST("/users", createUser)
	g.PUT("/users/:id", updateUser)
	g.DELETE("/users/:id", deleteUser)

	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "Echo is alive"})
	})

	log.Fatal(e.Start(":8080"))
}
