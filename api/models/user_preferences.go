package models

import "gorm.io/gorm"

type UserPreferences struct {
	gorm.Model
	UserID         string `json:"user_id" gorm:"type:uuid;uniqueIndex;not null"`
	ReceiptModel   string `json:"receipt_model" gorm:"default:'gemini-2.5-flash'"`
	AssistantModel string `json:"assistant_model" gorm:"default:'gemini-2.5-flash-lite'"`
	User           User   `json:"-" gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE"`
}
