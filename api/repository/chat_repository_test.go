package repository

import (
	"buybuddy-api/models"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestChatRepositorySeparatesConversationHistoryAndRecentQuestionsByDomain(t *testing.T) {
	dsn := os.Getenv("KNOWLEDGE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set KNOWLEDGE_TEST_DATABASE_URL to run PostgreSQL chat-domain integration coverage")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.ChatMessage{}); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	user := models.User{
		ID:       uuid.NewString(),
		Email:    uuid.NewString() + "@example.test",
		Name:     "Chat Domain Test",
		ClientID: uuid.NewString(),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	conversationID := uuid.NewString()
	t.Cleanup(func() {
		db.Unscoped().Where("user_id = ?", user.ID).Delete(&models.ChatMessage{})
		db.Unscoped().Where("id = ?", user.ID).Delete(&models.User{})
	})

	repo := NewChatRepository(db)
	for _, message := range []*models.ChatMessage{
		{ConversationID: conversationID, UserID: user.ID, Domain: models.ChatDomainReceipt, Role: "user", Content: "receipt question"},
		{ConversationID: conversationID, UserID: user.ID, Domain: models.ChatDomainKnowledge, Role: "user", Content: "knowledge question"},
	} {
		if err := repo.CreateMessage(message); err != nil {
			t.Fatalf("CreateMessage(%s) error = %v", message.Domain, err)
		}
	}

	legacyID := uuid.NewString()
	if err := db.Exec(
		`INSERT INTO chat_messages (id, conversation_id, user_id, role, content, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		legacyID, conversationID, user.ID, "user", "legacy receipt question", time.Now(),
	).Error; err != nil {
		t.Fatalf("insert legacy message using database default: %v", err)
	}

	receiptHistory, err := repo.GetConversationHistory(conversationID, user.ID, models.ChatDomainReceipt)
	if err != nil {
		t.Fatalf("receipt history error = %v", err)
	}
	knowledgeHistory, err := repo.GetConversationHistory(conversationID, user.ID, models.ChatDomainKnowledge)
	if err != nil {
		t.Fatalf("knowledge history error = %v", err)
	}
	if len(receiptHistory) != 2 || len(knowledgeHistory) != 1 {
		t.Fatalf("domain history lengths = receipt %d, knowledge %d; want 2/1", len(receiptHistory), len(knowledgeHistory))
	}
	if receiptHistory[0].Domain != models.ChatDomainReceipt || knowledgeHistory[0].Domain != models.ChatDomainKnowledge {
		t.Fatalf("unexpected domain history: %#v / %#v", receiptHistory, knowledgeHistory)
	}

	receiptQuestions, err := repo.GetRecentUserQuestions(user.ID, models.ChatDomainReceipt, 20)
	if err != nil {
		t.Fatalf("receipt questions error = %v", err)
	}
	knowledgeQuestions, err := repo.GetRecentUserQuestions(user.ID, models.ChatDomainKnowledge, 20)
	if err != nil {
		t.Fatalf("knowledge questions error = %v", err)
	}
	if len(receiptQuestions) != 2 || len(knowledgeQuestions) != 1 {
		t.Fatalf("recent question lengths = receipt %d, knowledge %d; want 2/1", len(receiptQuestions), len(knowledgeQuestions))
	}
}

func TestChatRepositoryQueriesAlwaysIncludeExplicitDomainScope(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=localhost user=test dbname=test sslmode=disable",
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("create dry-run database: %v", err)
	}

	conversationSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return chatConversationScope(tx, "conversation", "user", models.ChatDomainKnowledge).
			Find(&[]models.ChatMessage{})
	})
	recentSQL := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return recentChatQuestionScope(tx, "user", models.ChatDomainReceipt).
			Find(&[]models.ChatMessage{})
	})
	for name, query := range map[string]string{
		"conversation": conversationSQL,
		"recent":       recentSQL,
	} {
		if !strings.Contains(query, "domain") {
			t.Fatalf("%s query is not domain scoped: %s", name, query)
		}
	}
	if !strings.Contains(conversationSQL, models.ChatDomainKnowledge) {
		t.Fatalf("conversation query does not select knowledge domain: %s", conversationSQL)
	}
	if !strings.Contains(recentSQL, models.ChatDomainReceipt) {
		t.Fatalf("recent question query does not select receipt domain: %s", recentSQL)
	}
}
