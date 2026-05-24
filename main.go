package main

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
)

// User is a simple in-memory data model. No database — just a map.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// store is our in-memory "database". sync.RWMutex makes concurrent reads safe.
var (
	store = map[string]User{}
	mu    sync.RWMutex
)

// ── Handlers ──────────────────────────────────────────────────────────────────

// GET /api/v1/users
// Supports ?search=<name> as an optional filter.
func getUsers(c *echo.Context) error {
	search := c.QueryParam("search") // reads ?search=alice (empty string if absent)

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
	id := c.Param("id") // reads the :id segment from the URL path

	mu.RLock()
	u, ok := store[id]
	mu.RUnlock()

	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.JSON(http.StatusOK, u)
}

// POST /api/v1/users  (stub — real binding comes in Step 3)
func createUser(c *echo.Context) error {
	return c.JSON(http.StatusCreated, map[string]string{"message": "createUser stub"})
}

// PUT /api/v1/users/:id  (stub)
func updateUser(c *echo.Context) error {
	id := c.Param("id")
	return c.JSON(http.StatusOK, map[string]string{"message": "updateUser stub", "id": id})
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

	// A route group shares a URL prefix. Every route registered on g
	// automatically gets /api/v1 prepended.
	// Groups can also carry their own middleware (we'll use this in Step 5).
	g := e.Group("/api/v1")
	g.GET("/users", getUsers)
	g.GET("/users/:id", getUserByID)
	g.POST("/users", createUser)
	g.PUT("/users/:id", updateUser)
	g.DELETE("/users/:id", deleteUser)

	// Keep the health-check route from Step 1
	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "Echo is alive"})
	})

	log.Fatal(e.Start(":8080"))
}
