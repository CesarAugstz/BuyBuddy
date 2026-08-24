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

func (r *ChatRepository) GetConversationContext(conversationID string, userID string, limit int) ([]models.ChatMessage, error) {
	if limit <= 0 || limit > 30 {
		limit = 20
	}
	var messages []models.ChatMessage
	err := r.db.Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (r *ChatRepository) GetRecentUserQuestions(userID string, limit int) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := r.db.Where("user_id = ? AND role = ?", userID, "user").
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}
