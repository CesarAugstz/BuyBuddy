package handlers

import (
	"buybuddy-api/models"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAvailableModelsExposeDefaultsAndPricing(t *testing.T) {
	e := echo.New()
	recorder := httptest.NewRecorder()
	context := e.NewContext(httptest.NewRequest("GET", "/preferences/models", nil), recorder)
	handler := NewPreferencesHandler(nil)

	if err := handler.GetAvailableModels(context); err != nil {
		t.Fatalf("GetAvailableModels() error = %v", err)
	}

	var response struct {
		ReceiptModels   []map[string]string `json:"receipt_models"`
		AssistantModels []map[string]string `json:"assistant_models"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ReceiptModels[0]["id"] != models.DefaultReceiptModel {
		t.Errorf("default receipt model = %q", response.ReceiptModels[0]["id"])
	}
	if response.AssistantModels[0]["id"] != models.DefaultAssistantModel {
		t.Errorf("default assistant model = %q", response.AssistantModels[0]["id"])
	}

	for _, modelList := range [][]map[string]string{response.ReceiptModels, response.AssistantModels} {
		for _, model := range modelList {
			if model["pricing"] == "" {
				t.Errorf("model %q has no pricing", model["id"])
			}
			if model["id"] == "gemini-2.0-flash" {
				t.Error("shut-down Gemini 2.0 Flash is still selectable")
			}
		}
	}
}
