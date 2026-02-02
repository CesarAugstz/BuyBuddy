package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Email     string         `gorm:"uniqueIndex;not null" json:"email"`
	Name      string         `gorm:"not null" json:"name"`
	PhotoURL  string         `json:"photoUrl"`
	ClientID  string         `gorm:"uniqueIndex" json:"clientId"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type LoginRequest struct {
	IDToken     string `json:"idToken"`
	AccessToken string `json:"accessToken"`
	Email       string `json:"email" validate:"required,email"`
	Name        string `json:"name" validate:"required"`
	PhotoURL    string `json:"photoUrl"`
	ClientID    string `json:"clientId"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user,omitempty"`
}

type Session struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    string         `gorm:"type:uuid;not null;index" json:"userId"`
	Token     string         `gorm:"uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time      `gorm:"not null;default:NOW() + INTERVAL '7 days'" json:"expiresAt"`
	CreatedAt time.Time      `json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
