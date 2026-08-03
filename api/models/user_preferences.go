package models

import "gorm.io/gorm"

const (
	DefaultReceiptModel   = "gemini-3.6-flash"
	DefaultAssistantModel = "gemini-3.5-flash-lite"
)

var supportedGeminiModels = map[string]struct{}{
	"gemini-3.6-flash":      {},
	"gemini-3.5-flash":      {},
	"gemini-3.5-flash-lite": {},
	"gemini-2.5-pro":        {},
	"gemini-2.5-flash":      {},
	"gemini-2.5-flash-lite": {},
}

func IsSupportedGeminiModel(model string) bool {
	_, supported := supportedGeminiModels[model]
	return supported
}

type UserPreferences struct {
	gorm.Model
	UserID         string `json:"user_id" gorm:"type:uuid;uniqueIndex;not null"`
	ReceiptModel   string `json:"receipt_model" gorm:"default:'gemini-3.6-flash'"`
	AssistantModel string `json:"assistant_model" gorm:"default:'gemini-3.5-flash-lite'"`
	User           User   `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}
