package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"user-api/models"
	"user-api/store"
)

// validationErrors converts validator.ValidationErrors into a structured
// JSON 400 response: {"errors": {"field": "failing-rule", ...}}
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

// GET /api/v1/users
func GetUsers(c *echo.Context) error {
	// Extract caller identity from the JWT the middleware stored on context.
	token := c.Get("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	callerEmail, _ := claims["email"].(string)
	fmt.Printf("[GetUsers] called by: %s\n", callerEmail)

	result := store.GetAll(c.QueryParam("search"))
	return c.JSON(http.StatusOK, result)
}

// GET /api/v1/users/:id
func GetUserByID(c *echo.Context) error {
	u, ok := store.GetByID(c.Param("id"))
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.JSON(http.StatusOK, u)
}

// POST /api/v1/users
func CreateUser(c *echo.Context) error {
	var u models.User
	if err := c.Bind(&u); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if err := c.Validate(&u); err != nil {
		return validationErrors(c, err)
	}
	u.ID = uuid.NewString()
	store.Create(u)
	return c.JSON(http.StatusCreated, u)
}

// PUT /api/v1/users/:id
func UpdateUser(c *echo.Context) error {
	id := c.Param("id")
	existing, ok := store.GetByID(id)
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
	store.Update(id, updated)
	return c.JSON(http.StatusOK, updated)
}

// DELETE /api/v1/users/:id
func DeleteUser(c *echo.Context) error {
	if !store.Delete(c.Param("id")) {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.NoContent(http.StatusNoContent)
}
