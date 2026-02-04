package models

import (
	"time"

	"gorm.io/gorm"
)

type ShoppingList struct {
	ID          string              `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Title       string              `gorm:"not null" json:"title"`
	Description string              `json:"description,omitempty"`
	OwnerID     string              `gorm:"type:uuid;not null;index" json:"ownerId"`
	Owner       *User               `gorm:"foreignKey:OwnerID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"owner,omitempty"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt      `gorm:"index" json:"-"`
	Items       []ShoppingListItem  `gorm:"foreignKey:ListID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"items,omitempty"`
	Shares      []ShoppingListShare `gorm:"foreignKey:ListID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"shares,omitempty"`
}

type ShoppingListItem struct {
	ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ListID    string         `gorm:"type:uuid;not null;index" json:"listId"`
	List      *ShoppingList  `gorm:"foreignKey:ListID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Name      string         `gorm:"not null" json:"name"`
	Quantity  float64        `gorm:"not null;default:1" json:"quantity"`
	Unit      string         `gorm:"default:'un'" json:"unit"`
	IsChecked bool           `gorm:"default:false" json:"isChecked"`
	SortOrder int            `gorm:"default:0" json:"sortOrder"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type ShareStatus string

const (
	ShareStatusPending  ShareStatus = "pending"
	ShareStatusAccepted ShareStatus = "accepted"
	ShareStatusRejected ShareStatus = "rejected"
)

type ShoppingListShare struct {
	ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	ListID    string         `gorm:"type:uuid;not null;index" json:"listId"`
	List      *ShoppingList  `gorm:"foreignKey:ListID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	UserID    string         `gorm:"type:uuid;not null;index" json:"userId"`
	User      *User          `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
	InvitedBy string         `gorm:"type:uuid;not null" json:"invitedBy"`
	Inviter   *User          `gorm:"foreignKey:InvitedBy;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"inviter,omitempty"`
	Status    ShareStatus    `gorm:"type:varchar(20);default:'pending'" json:"status"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type CreateShoppingListRequest struct {
	Title       string `json:"title" validate:"required"`
	Description string `json:"description"`
}

type UpdateShoppingListRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type CreateShoppingListItemRequest struct {
	Name     string  `json:"name" validate:"required"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

type UpdateShoppingListItemRequest struct {
	Name      string  `json:"name"`
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
	IsChecked *bool   `json:"isChecked"`
	SortOrder *int    `json:"sortOrder"`
}

type ReorderItemsRequest struct {
	Items []struct {
		ID        string `json:"id"`
		SortOrder int    `json:"sortOrder"`
	} `json:"items" validate:"required"`
}

type ShareListRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type ShoppingListResponse struct {
	ShoppingList
	ItemCount       int  `json:"itemCount"`
	CheckedCount    int  `json:"checkedCount"`
	IsShared        bool `json:"isShared"`
	IsOwner         bool `json:"isOwner"`
	SharedWithCount int  `json:"sharedWithCount"`
}
