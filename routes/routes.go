package routes

import (
	"net/http"

	"github.com/labstack/echo/v5"
	echomw "github.com/labstack/echo/v5/middleware"

	"user-api/handlers"
	mw "user-api/middleware"
)

// Register wires all middleware and routes onto the Echo instance.
// Called once from main — keeping main.go as a pure composition root.
func Register(e *echo.Echo) {
	// ── Global middleware ────────────────────────────────────────────────────
	// Execution order: RequestID → Audit → RequestLogger → CORS → BodyLimit → handler
	e.Use(mw.RequestID)
	e.Use(mw.Audit)
	e.Use(echomw.RequestLogger())
	e.Use(echomw.CORS("*"))
	e.Use(echomw.BodyLimit(2 * 1024 * 1024))

	// ── Public routes ────────────────────────────────────────────────────────
	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "Echo is alive"})
	})
	e.POST("/login", handlers.Login)

	// ── Protected routes ─────────────────────────────────────────────────────
	// JWTAuth is scoped to this group only — /login stays public.
	g := e.Group("/api/v1", mw.JWTAuth)
	g.GET("/users", handlers.GetUsers)
	g.GET("/users/:id", handlers.GetUserByID)
	g.POST("/users", handlers.CreateUser)
	g.PUT("/users/:id", handlers.UpdateUser)
	g.DELETE("/users/:id", handlers.DeleteUser)
}
