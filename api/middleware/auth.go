package middleware

import (
	"easybuy-api/utils"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

func AuthMiddleware(jwtSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
			}

			token := parts[1]
			claims, err := utils.ValidateJWT(token, jwtSecret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
			}

			c.Set("userID", claims.UserID)
			c.Set("email", claims.Email)

			return next(c)
		}
	}
}
