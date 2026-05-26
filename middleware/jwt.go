package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"

	"user-api/config"
)

func JWTAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing or malformed token")
		}

		token, err := jwt.ParseWithClaims(
			strings.TrimPrefix(authHeader, "Bearer "),
			jwt.MapClaims{},
			func(t *jwt.Token) (any, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return config.JWTSecret, nil
			},
		)
		if err != nil || !token.Valid {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
		}

		c.Set("user", token)
		return next(c)
	}
}
