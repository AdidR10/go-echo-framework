package main

import (
	"log"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"

	"user-api/models"
	"user-api/routes"
)

func main() {
	e := echo.New()

	// Validator — lets handlers call c.Validate(&struct{})
	e.Validator = &models.CustomValidator{V: validator.New()}

	// Centralized error handler.
	// Every echo.NewHTTPError() and every unhandled error lands here.
	// This guarantees ALL error responses have the same JSON shape:
	//   {"error": "<message>", "status": <code>}
	//
	// Echo v5 signature: func(c *echo.Context, err error)
	// Note the parameters are swapped vs v4.
	e.HTTPErrorHandler = func(c *echo.Context, err error) {
		code := http.StatusInternalServerError
		msg := "internal server error"

		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			msg = he.Message // string in Echo v5
		}

		// Swallow the write error — nothing we can do if the client disconnected.
		_ = c.JSON(code, map[string]any{"error": msg, "status": code})
	}

	routes.Register(e)

	log.Fatal(e.Start(":8080"))
}
