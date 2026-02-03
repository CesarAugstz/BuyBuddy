package handlers

import (
	"buybuddy-api/config"
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"buybuddy-api/utils"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type AssistantHandler struct {
	cfg          *config.Config
	receiptRepo  *repository.ReceiptRepository
	categoryRepo *repository.CategoryRepository
	chatRepo     *repository.ChatRepository
}

func NewAssistantHandler(cfg *config.Config, receiptRepo *repository.ReceiptRepository, categoryRepo *repository.CategoryRepository, chatRepo *repository.ChatRepository) *AssistantHandler {
	return &AssistantHandler{
		cfg:          cfg,
		receiptRepo:  receiptRepo,
		categoryRepo: categoryRepo,
		chatRepo:     chatRepo,
	}
}

func (h *AssistantHandler) AskQuestion(c echo.Context) error {
	userID := c.Get("userID").(string)

	var req models.AssistantRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Question == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "question is required")
	}

	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	conversationHistory, err := h.chatRepo.GetConversationHistory(conversationID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch conversation history")
	}

	receipts, err := h.receiptRepo.GetByUserID(userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch receipts")
	}

	answer, err := utils.AskShoppingAssistant(c.Request().Context(), req.Question, receipts, conversationHistory, h.cfg.GeminiAPIKey)
	if err != nil {
		fmt.Println("Assistant error:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"message": "Failed to get answer from assistant",
			"error":   err.Error(),
		})
	}

	userMessage := &models.ChatMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Role:           "user",
		Content:        req.Question,
	}
	if err := h.chatRepo.CreateMessage(userMessage); err != nil {
		fmt.Println("Failed to save user message:", err)
	}

	assistantMessage := &models.ChatMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Role:           "assistant",
		Content:        answer,
	}
	if err := h.chatRepo.CreateMessage(assistantMessage); err != nil {
		fmt.Println("Failed to save assistant message:", err)
	}

	return c.JSON(http.StatusOK, models.AssistantResponse{
		Answer:         answer,
		ConversationID: conversationID,
	})
}

func (h *AssistantHandler) GetConversationHistory(c echo.Context) error {
	userID := c.Get("userID").(string)
	conversationID := c.Param("conversationId")

	if conversationID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "conversationId is required")
	}

	messages, err := h.chatRepo.GetConversationHistory(conversationID, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch conversation history")
	}

	return c.JSON(http.StatusOK, messages)
}
