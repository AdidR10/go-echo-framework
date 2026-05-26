package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

func Audit(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		start := time.Now()
		reqID := fmt.Sprintf("%v", c.Get("requestID"))

		fmt.Printf("[Audit]      IN  id=%-36s  %s %s\n",
			reqID, c.Request().Method, c.Request().URL.Path)

		err := next(c)

		status := http.StatusOK
		if res, err2 := echo.UnwrapResponse(c.Response()); err2 == nil {
			status = res.Status
		}
		fmt.Printf("[Audit]      OUT id=%-36s  status=%d  duration=%s\n",
			reqID, status, time.Since(start))

		return err
	}
}
