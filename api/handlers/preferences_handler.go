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
	if req.ReceiptModel != "" && !models.IsSupportedGeminiModel(req.ReceiptModel) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unsupported receipt model"})
	}
	if req.AssistantModel != "" && !models.IsSupportedGeminiModel(req.AssistantModel) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unsupported assistant model"})
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
			{"id": "gemini-3.6-flash", "name": "Gemini 3.6 Flash", "description": "Best extraction quality (default)", "pricing": "Paid: $1.50 input / $7.50 output per 1M tokens"},
			{"id": "gemini-3.5-flash", "name": "Gemini 3.5 Flash", "description": "Fast frontier intelligence", "pricing": "Paid: $1.50 input / $9.00 output per 1M tokens"},
			{"id": "gemini-3.5-flash-lite", "name": "Gemini 3.5 Flash-Lite", "description": "Cost-efficient high-volume processing", "pricing": "Paid: $0.30 input / $2.50 output per 1M tokens"},
			{"id": "gemini-2.5-pro", "name": "Gemini 2.5 Pro", "description": "Complex reasoning", "pricing": "Paid: $1.25 input / $10.00 output per 1M tokens (up to 200k)"},
			{"id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "description": "Previous-generation balanced model", "pricing": "Paid: $0.30 input / $2.50 output per 1M tokens"},
			{"id": "gemini-2.5-flash-lite", "name": "Gemini 2.5 Flash-Lite", "description": "Previous-generation low-cost model", "pricing": "Paid: $0.10 input / $0.40 output per 1M tokens"},
		},
		"assistant_models": []map[string]string{
			{"id": "gemini-3.5-flash-lite", "name": "Gemini 3.5 Flash-Lite", "description": "Fast, cost-efficient chat (default)", "pricing": "Paid: $0.30 input / $2.50 output per 1M tokens"},
			{"id": "gemini-3.6-flash", "name": "Gemini 3.6 Flash", "description": "Most intelligent Flash model", "pricing": "Paid: $1.50 input / $7.50 output per 1M tokens"},
			{"id": "gemini-3.5-flash", "name": "Gemini 3.5 Flash", "description": "Fast frontier intelligence", "pricing": "Paid: $1.50 input / $9.00 output per 1M tokens"},
			{"id": "gemini-2.5-pro", "name": "Gemini 2.5 Pro", "description": "Complex reasoning", "pricing": "Paid: $1.25 input / $10.00 output per 1M tokens (up to 200k)"},
			{"id": "gemini-2.5-flash", "name": "Gemini 2.5 Flash", "description": "Previous-generation balanced model", "pricing": "Paid: $0.30 input / $2.50 output per 1M tokens"},
			{"id": "gemini-2.5-flash-lite", "name": "Gemini 2.5 Flash-Lite", "description": "Previous-generation low-cost model", "pricing": "Paid: $0.10 input / $0.40 output per 1M tokens"},
		},
	}

	return c.JSON(http.StatusOK, models)
}
