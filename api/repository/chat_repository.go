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
	message.Domain = models.NormalizeChatDomain(message.Domain)
	return r.db.Create(message).Error
}

func (r *ChatRepository) GetConversationHistory(conversationID string, userID string, domain string) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := chatConversationScope(r.db, conversationID, userID, domain).
		Order("created_at ASC").
		Find(&messages).Error
	return messages, err
}

func (r *ChatRepository) GetConversationContext(conversationID string, userID string, domain string, limit int) ([]models.ChatMessage, error) {
	if limit <= 0 || limit > 30 {
		limit = 20
	}
	var messages []models.ChatMessage
	err := chatConversationScope(r.db, conversationID, userID, domain).
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

func (r *ChatRepository) GetRecentUserQuestions(userID string, domain string, limit int) ([]models.ChatMessage, error) {
	var messages []models.ChatMessage
	err := recentChatQuestionScope(r.db, userID, domain).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	return messages, err
}

func chatConversationScope(db *gorm.DB, conversationID, userID, domain string) *gorm.DB {
	return db.Where(
		"conversation_id = ? AND user_id = ? AND domain = ?",
		conversationID,
		userID,
		models.NormalizeChatDomain(domain),
	)
}

func recentChatQuestionScope(db *gorm.DB, userID, domain string) *gorm.DB {
	return db.Where(
		"user_id = ? AND role = ? AND domain = ?",
		userID,
		"user",
		models.NormalizeChatDomain(domain),
	)
}
