package handlers

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"

	"user-api/config"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// POST /login
func Login(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Email != "admin@example.com" || req.Password != "secret" {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}

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

	signed, err := token.SignedString(config.JWTSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not sign token")
	}

	return c.JSON(http.StatusOK, map[string]string{"token": signed})
}
