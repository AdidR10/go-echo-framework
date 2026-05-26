package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

// jwtSecret is the symmetric key used to sign and verify tokens.
// In production this comes from an environment variable, never hardcoded.
var jwtSecret = []byte("super-secret-key")

// ── Model ─────────────────────────────────────────────────────────────────────

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"  validate:"required,min=2"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age"   validate:"gte=0,lte=130"`
}

// ── Validator ─────────────────────────────────────────────────────────────────

type CustomValidator struct {
	v *validator.Validate
}

func (cv *CustomValidator) Validate(i any) error {
	return cv.v.Struct(i)
}

func validationErrors(c *echo.Context, err error) error {
	errs, ok := err.(validator.ValidationErrors)
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid input")
	}
	fields := make(map[string]string, len(errs))
	for _, fe := range errs {
		fields[strings.ToLower(fe.Field())] = fe.Tag()
	}
	return c.JSON(http.StatusBadRequest, map[string]any{"errors": fields})
}

// ── Store ─────────────────────────────────────────────────────────────────────

var (
	store = map[string]User{}
	mu    sync.RWMutex
)

// ── Custom Middleware ─────────────────────────────────────────────────────────

func RequestID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		id := c.Request().Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		c.Set("requestID", id)
		c.Response().Header().Set("X-Request-ID", id)
		fmt.Printf("[RequestID]  IN  id=%-36s  %s %s\n",
			id, c.Request().Method, c.Request().URL.Path)
		return next(c)
	}
}

func Audit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		start := time.Now()
		reqID := fmt.Sprintf("%v", c.Get("requestID"))
		fmt.Printf("[Audit]      IN  id=%-36s  %s %s\n",
			reqID, c.Request().Method, c.Request().URL.Path)

		err := next(c)

		elapsed := time.Since(start)
		status := http.StatusOK
		if res, err2 := echo.UnwrapResponse(c.Response()); err2 == nil {
			status = res.Status
		}
		fmt.Printf("[Audit]      OUT id=%-36s  status=%d  duration=%s\n",
			reqID, status, elapsed)
		return err
	}
}

// JWTAuth is our hand-rolled JWT middleware.
// It reads the Authorization header, validates the token, and stores
// the parsed *jwt.Token on the context under the key "user" so handlers
// can extract claims from it.
func JWTAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")

		// Header must be exactly "Bearer <token>"
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing or malformed token")
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// jwt.ParseWithClaims validates:
		//   • the signature (using jwtSecret)
		//   • the exp claim (token not expired)
		//   • the algorithm (we only accept HS256 via the keyFunc)
		token, err := jwt.ParseWithClaims(
			tokenString,
			jwt.MapClaims{}, // the shape we expect claims in
			func(t *jwt.Token) (any, error) {
				// Guard against algorithm-switching attacks:
				// reject any token that was not signed with HS256.
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return jwtSecret, nil
			},
		)
		if err != nil || !token.Valid {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
		}

		// Store the parsed token so any handler in this group can read claims.
		c.Set("user", token)
		return next(c)
	}
}

// ── Auth Handler ──────────────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// POST /login
// Validates credentials against a hardcoded test user, then signs and
// returns a JWT valid for 1 hour.
func login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Hardcoded credentials — in a real app you'd query a database.
	if req.Email != "admin@example.com" || req.Password != "secret" {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

	// Build the claims.
	// RegisteredClaims covers the standard JWT fields (sub, exp, iat, etc.).
	// We embed it in a custom struct alongside our own fields.
	type claims struct {
		Email string `json:"email"`
		jwt.RegisteredClaims
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Email: req.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   req.Email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	})

	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not sign token")
	}

	return c.JSON(http.StatusOK, map[string]string{"token": signed})
}

// ── User Handlers ─────────────────────────────────────────────────────────────

// GET /api/v1/users
// Demonstrates reading claims from the token that JWTAuth placed on the context.
func getUsers(c *echo.Context) error {
	// Retrieve the token the JWTAuth middleware stored.
	token := c.Get("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)

	// MapClaims["email"] is the field we put in the token during login.
	callerEmail, _ := claims["email"].(string)
	fmt.Printf("[getUsers] called by: %s\n", callerEmail)

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

func createUser(c *echo.Context) error {
	var u User
	if err := c.Bind(&u); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&u); err != nil {
		return validationErrors(c, err)
	}
	u.ID = uuid.NewString()
	mu.Lock()
	store[u.ID] = u
	mu.Unlock()
	return c.JSON(http.StatusCreated, u)
}

func updateUser(c *echo.Context) error {
	id := c.Param("id")
	mu.RLock()
	existing, ok := store[id]
	mu.RUnlock()
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	updated := existing
	if err := c.Bind(&updated); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&updated); err != nil {
		return validationErrors(c, err)
	}
	updated.ID = id
	mu.Lock()
	store[id] = updated
	mu.Unlock()
	return c.JSON(http.StatusOK, updated)
}

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
	e.Validator = &CustomValidator{v: validator.New()}

	// Global middleware — runs on every route including /login
	e.Use(RequestID)
	e.Use(Audit)
	e.Use(middleware.RequestLogger())
	e.Use(middleware.CORS("*"))
	e.Use(middleware.BodyLimit(2 * 1024 * 1024))

	// Public route — no JWT required
	e.POST("/login", login)

	e.GET("/", func(c *echo.Context) error {
		reqID := fmt.Sprintf("%v", c.Get("requestID"))
		return c.JSON(http.StatusOK, map[string]string{
			"message":    "Echo is alive",
			"request_id": reqID,
		})
	})

	// Protected group — JWTAuth runs on every route under /api/v1.
	// Any request without a valid Bearer token is rejected with 401
	// before it ever reaches a handler.
	g := e.Group("/api/v1", JWTAuth)
	g.GET("/users", getUsers)
	g.GET("/users/:id", getUserByID)
	g.POST("/users", createUser)
	g.PUT("/users/:id", updateUser)
	g.DELETE("/users/:id", deleteUser)

	log.Fatal(e.Start(":8080"))
}
