package middleware

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

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
