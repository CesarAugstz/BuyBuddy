package handlers

import (
	"buybuddy-api/config"
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"buybuddy-api/utils"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type AssistantHandler struct {
	cfg          *config.Config
	receiptRepo  *repository.ReceiptRepository
	chatRepo     *repository.ChatRepository
	prefsRepo    *repository.PreferencesRepository
	categoryRepo *repository.CategoryRepository
}

func NewAssistantHandler(cfg *config.Config, receiptRepo *repository.ReceiptRepository, chatRepo *repository.ChatRepository, prefsRepo *repository.PreferencesRepository, categoryRepo *repository.CategoryRepository) *AssistantHandler {
	return &AssistantHandler{
		cfg:          cfg,
		receiptRepo:  receiptRepo,
		chatRepo:     chatRepo,
		prefsRepo:    prefsRepo,
		categoryRepo: categoryRepo,
	}
}

func (h *AssistantHandler) getFirstReceiptDate(userID string) *time.Time {
	cache := utils.GetFirstReceiptCache()
	if date, ok := cache.Get(userID); ok {
		return date
	}

	date, err := h.receiptRepo.GetFirstReceiptDate(userID)
	if err != nil {
		fmt.Println("Failed to get first receipt date:", err)
		return nil
	}

	cache.Set(userID, date)
	return date
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

	prefs, _ := h.prefsRepo.GetOrCreate(userID)
	assistantModel := prefs.AssistantModel
	if assistantModel == "" {
		assistantModel = "gemini-2.5-flash-lite"
	}

	firstReceiptDate := h.getFirstReceiptDate(userID)

	categories, _ := h.categoryRepo.GetAll()

	intent, err := utils.DetectIntentAndGenerateQuery(c.Request().Context(), req.Question, conversationHistory, firstReceiptDate, categories, h.cfg.GeminiAPIKey)
	if err != nil {
		fmt.Println("Intent detection error:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
			"message": "Failed to process question",
			"error":   err.Error(),
		})
	}

	var answer string

	if intent.Type == "direct" {
		answer = intent.Answer
	} else {
		var specificResults, generalResults []models.Receipt

		log.Printf("Specific query filters: %+v", intent.Specific)
		log.Printf("General query filters: %+v", intent.General)

		if intent.Specific != nil {
			specificResults, err = h.receiptRepo.QueryWithFilters(userID, intent.Specific, 30)
			if err != nil {
				fmt.Println("Specific query error:", err)
			}
		}

		if intent.General != nil {
			generalResults, err = h.receiptRepo.QueryWithFilters(userID, intent.General, 30)
			if err != nil {
				fmt.Println("General query error:", err)
			}
		}

		mergedResults := utils.MergeResults(specificResults, generalResults)
		compactReceipts := utils.FormatReceiptsCompact(mergedResults, intent.Specific)

		answer, err = utils.GenerateAnswer(c.Request().Context(), req.Question, compactReceipts, conversationHistory, h.cfg.GeminiAPIKey, assistantModel)
		if err != nil {
			fmt.Println("Answer generation error:", err)
			return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
				"message": "Failed to get answer from assistant",
				"error":   err.Error(),
			})
		}
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

	log.Printf("User %s asked: %s | Assistant answered: %s", userID, req.Question, answer)

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
