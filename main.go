package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

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

// RequestID is a middleware that stamps every request with a unique ID.
//
// Middleware in Echo is simply a function that wraps a handler:
//
//	func(next HandlerFunc) HandlerFunc
//
// The inner function is the actual per-request logic. Calling next(c) passes
// control down the chain — anything before that call runs on the way IN,
// anything after runs on the way OUT (after the handler has responded).
func RequestID(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		// Honour an existing ID if the client (or a load balancer) already set one.
		id := c.Request().Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}

		// Store on the context so any handler can read it with c.Get("requestID").
		c.Set("requestID", id)

		// Echo writes response headers lazily, so we must set them before
		// calling next — once the handler calls c.JSON the headers are flushed.
		c.Response().Header().Set("X-Request-ID", id)

		fmt.Printf("[RequestID]  IN  id=%-36s  %s %s\n",
			id, c.Request().Method, c.Request().URL.Path)

		// Hand off to the next middleware/handler in the chain.
		return next(c)
	}
}

// Audit is the second custom middleware. It runs after RequestID, so by the time
// it executes the context already has "requestID" in it — that's the chain at work.
//
// The key pattern to notice: code BEFORE next(c) runs on the way IN (request),
// code AFTER next(c) runs on the way OUT (response). The handler itself sits
// between those two halves.
//
//	[RequestID] ──► [Audit ──► handler ──► Audit] ──► [RequestID]
//	               ▲ before              ▲ after
func Audit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		// ── WAY IN ──────────────────────────────────────────────────────────
		start := time.Now()

		// c.Get reads what RequestID already stored — no parameters passed,
		// just a shared context. This proves the chain shares state.
		reqID := fmt.Sprintf("%v", c.Get("requestID"))

		fmt.Printf("[Audit]      IN  id=%-36s  %s %s\n",
			reqID, c.Request().Method, c.Request().URL.Path)

		// ── CALL THE NEXT LAYER (eventually reaches the handler) ─────────────
		err := next(c)

		// ── WAY OUT ─────────────────────────────────────────────────────────
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

// ── Handlers ──────────────────────────────────────────────────────────────────

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

	// ── Global middleware ────────────────────────────────────────────────────
	//
	// e.Use registers middleware that runs on EVERY request, in registration order.
	// Think of it as a stack of wrappers around your handlers:
	//
	//   RequestID → RequestLogger → CORS → BodyLimit → [handler]
	//
	// Each layer calls next(c) to pass control inward, then gets control back
	// when the inner layers return.

	// Our custom middleware — runs first so every subsequent log line can include the ID.
	e.Use(RequestID)
	// Audit runs second: it can read "requestID" set by RequestID, wraps the
	// handler call, and logs the response status on the way out.
	e.Use(Audit)

	// RequestLogger logs method, path, status, latency after the handler returns.
	// In Echo v5 this replaces the old middleware.Logger().
	e.Use(middleware.RequestLogger())

	// CORS adds the Access-Control-Allow-Origin header to every response.
	// Passing "*" allows all origins — tighten this for production.
	e.Use(middleware.CORS("*"))

	// BodyLimit caps the request body at 2 MB. Requests over the limit get a
	// 413 before they even reach your handler.
	e.Use(middleware.BodyLimit(2 * 1024 * 1024)) // 2 MB in bytes

	// ── Routes ──────────────────────────────────────────────────────────────

	g := e.Group("/api/v1")
	g.GET("/users", getUsers)
	g.GET("/users/:id", getUserByID)
	g.POST("/users", createUser)
	g.PUT("/users/:id", updateUser)
	g.DELETE("/users/:id", deleteUser)

	e.GET("/", func(c *echo.Context) error {
		// Demo: read the request ID our middleware stored on the context.
		reqID := fmt.Sprintf("%v", c.Get("requestID"))
		return c.JSON(http.StatusOK, map[string]string{
			"message":    "Echo is alive",
			"request_id": reqID,
		})
	})

	log.Fatal(e.Start(":8080"))
}
