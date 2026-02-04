package handlers

import (
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"net/http"

	"github.com/labstack/echo/v4"
)

type PreferencesHandler struct {
	prefsRepo *repository.PreferencesRepository
}

func NewPreferencesHandler(prefsRepo *repository.PreferencesRepository) *PreferencesHandler {
	return &PreferencesHandler{prefsRepo: prefsRepo}
}

func (h *PreferencesHandler) GetPreferences(c echo.Context) error {
	userID := c.Get("userID").(string)

	prefs, err := h.prefsRepo.GetOrCreate(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get preferences"})
	}

	return c.JSON(http.StatusOK, prefs)
}

func (h *PreferencesHandler) UpdatePreferences(c echo.Context) error {
	userID := c.Get("userID").(string)

	var req models.UserPreferences
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	prefs, err := h.prefsRepo.GetOrCreate(userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get preferences"})
	}

	if req.ReceiptModel != "" {
		prefs.ReceiptModel = req.ReceiptModel
	}
	if req.AssistantModel != "" {
		prefs.AssistantModel = req.AssistantModel
	}

	if err := h.prefsRepo.Update(prefs); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update preferences"})
	}

	return c.JSON(http.StatusOK, prefs)
}

func (h *PreferencesHandler) GetAvailableModels(c echo.Context) error {
	models := map[string]interface{}{
		"receipt_models": []map[string]string{
			{"id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "description": "Latest and fastest (default)"},
			{"id": "gemini-2.5-pro", "name": "Gemini 2.5 Pro", "description": "Most capable"},
			{"id": "gemini-2.5-flash-lite", "name": "Gemini 2.5 Flash Lite", "description": "Lightweight and fast"},
			{"id": "gemini-2.0-flash", "name": "Gemini 2.0 Flash", "description": "Reliable multimodal"},
		},
		"assistant_models": []map[string]string{
			{"id": "gemini-2.5-flash-lite", "name": "Gemini 2.5 Flash Lite", "description": "Quick responses (default)"},
			{"id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "description": "Latest and fastest"},
			{"id": "gemini-2.5-pro", "name": "Gemini 2.5 Pro", "description": "Most intelligent"},
			{"id": "gemini-2.0-flash", "name": "Gemini 2.0 Flash", "description": "Balanced performance"},
		},
	}

	return c.JSON(http.StatusOK, models)
}
