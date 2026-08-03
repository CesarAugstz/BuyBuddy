package models

import "testing"

func TestDefaultModelsAreSupported(t *testing.T) {
	for _, model := range []string{DefaultReceiptModel, DefaultAssistantModel} {
		if !IsSupportedGeminiModel(model) {
			t.Errorf("default model %q is not supported", model)
		}
	}
}

func TestDeprecatedGeminiModelIsNotSupported(t *testing.T) {
	if IsSupportedGeminiModel("gemini-2.0-flash") {
		t.Error("deprecated Gemini 2.0 Flash should not be supported")
	}
}
