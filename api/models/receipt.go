package models

import (
	"time"

	"gorm.io/gorm"
)

type Receipt struct {
	ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID    string         `gorm:"type:uuid;not null;index" json:"userId"`
	Company   string         `gorm:"not null" json:"company"`
	Total     float64        `gorm:"not null" json:"total"`
	AccessKey string         `gorm:"uniqueIndex;size:44" json:"accessKey,omitempty"`
	ImageURL  string         `json:"imageUrl,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Items     []ReceiptItem  `gorm:"foreignKey:ReceiptID" json:"items,omitempty"`
}

type ReceiptItem struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	ReceiptID     string         `gorm:"type:uuid;not null;index" json:"receiptId"`
	Name          string         `gorm:"not null" json:"name"`
	Brand         string         `json:"brand,omitempty"`
	Quantity      float64        `gorm:"not null;default:1" json:"quantity"`
	Unit          string         `gorm:"default:un" json:"unit"`
	UnitPrice     float64        `gorm:"not null" json:"unitPrice"`
	TotalPrice    float64        `gorm:"not null" json:"totalPrice"`
	CategoryID    *uint          `gorm:"index" json:"categoryId,omitempty"`
	SubcategoryID *uint          `gorm:"index" json:"subcategoryId,omitempty"`
	Barcode       string         `json:"barcode,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Category      *Category      `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Subcategory   *Subcategory   `gorm:"foreignKey:SubcategoryID" json:"subcategory,omitempty"`
}

type Category struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Name          string         `gorm:"uniqueIndex;not null" json:"name"`
	Description   string         `json:"description,omitempty"`
	Icon          string         `json:"icon,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Subcategories []Subcategory  `gorm:"foreignKey:CategoryID" json:"subcategories,omitempty"`
}

type Subcategory struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CategoryID  uint           `gorm:"not null;index" json:"categoryId"`
	Name        string         `gorm:"not null" json:"name"`
	Description string         `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type ProcessReceiptRequest struct {
	Image string `json:"image" validate:"required"`
}

type ProcessReceiptResponse struct {
	Company   string                   `json:"company"`
	Total     float64                  `json:"total"`
	AccessKey string                   `json:"accessKey,omitempty"`
	Items     []map[string]interface{} `json:"items"`
}

type SaveReceiptRequest struct {
	Company   string                   `json:"company" validate:"required"`
	Total     float64                  `json:"total" validate:"required"`
	AccessKey string                   `json:"accessKey,omitempty"`
	Items     []map[string]interface{} `json:"items" validate:"required"`
}

type AssistantRequest struct {
	Question       string `json:"question" validate:"required"`
	ConversationID string `json:"conversationId,omitempty"`
}

type AssistantResponse struct {
	Answer         string `json:"answer"`
	ConversationID string `json:"conversationId"`
}

type ChatMessage struct {
	ID             string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ConversationID string         `gorm:"type:uuid;not null;index" json:"conversationId"`
	UserID         string         `gorm:"type:uuid;not null;index" json:"userId"`
	Role           string         `gorm:"not null" json:"role"`
	Content        string         `gorm:"type:text;not null" json:"content"`
	CreatedAt      time.Time      `json:"createdAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}
