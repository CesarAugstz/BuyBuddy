package handlers

import (
	"easybuy-api/config"
	"easybuy-api/models"
	"easybuy-api/repository"
	"easybuy-api/utils"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	cfg      *config.Config
	userRepo *repository.UserRepository
}

func NewAuthHandler(cfg *config.Config, userRepo *repository.UserRepository) *AuthHandler {
	return &AuthHandler{
		cfg:      cfg,
		userRepo: userRepo,
	}
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req models.LoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	fmt.Println("Login request:", req)
	if req.Email == "" || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing required fields")
	}

	if h.cfg.GoogleClientID != "" && req.IDToken != "" {
		tokenInfo, err := utils.VerifyGoogleIDToken(c.Request().Context(), req.IDToken, h.cfg.GoogleClientID)
		if err != nil {
			fmt.Println("Google token verification failed:", err)
		} else {
			if tokenInfo.Email != req.Email {
				return echo.NewHTTPError(http.StatusUnauthorized, "email mismatch")
			}

			if req.PhotoURL == "" {
				req.PhotoURL = tokenInfo.Picture
			}
		}
	}

	clientID := req.ClientID
	if clientID == "" {
		clientID = "google"
	}

	user, err := h.userRepo.CreateOrUpdateUser(req.Email, req.Name, req.PhotoURL, clientID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create user")
	}

	token, err := utils.GenerateJWT(user.ID, user.Email, h.cfg.JWTSecret)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}

	if err := h.userRepo.CreateSession(user.ID, token); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session")
	}

	return c.JSON(http.StatusOK, models.LoginResponse{
		Token: token,
		User:  user,
	})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return c.JSON(http.StatusOK, map[string]string{"message": "logged out"})
	}

	token := ""
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		token = authHeader[7:]
	}

	if token != "" {
		h.userRepo.DeleteSession(token)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Verify(c echo.Context) error {
	userID := c.Get("userID").(string)
	email := c.Get("email").(string)

	user, err := h.userRepo.GetUserByID(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
		"user":  user,
		"email": email,
	})
}
