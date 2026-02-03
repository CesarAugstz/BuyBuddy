package repository

import (
	"buybuddy-api/models"

	"gorm.io/gorm"
)

type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) CreateMessage(message *models.ChatMessage) error {
	return r.db.Create(message).Error
}

func (r *ChatRepository) GetConversationHistory(conversationID string, userID string) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

func (r *ChatRepository) DeleteConversation(conversationID string, userID string) error {
	return r.db.Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Delete(&models.ChatMessage{}).Error
}
