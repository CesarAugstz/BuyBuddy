package handlers

import (
	"buybuddy-api/config"
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"buybuddy-api/utils"
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type KnowledgeAssistantHandler struct {
	cfg            *config.Config
	chatRepo       knowledgeAssistantChatRepository
	prefsRepo      knowledgeAssistantPreferencesRepository
	knowledgeRepo  knowledgeAssistantRepository
	organizer      KnowledgeOrganizer
	detectIntent   knowledgeIntentDetector
	generateAnswer knowledgeAnswerGenerator
}

type knowledgeAssistantChatRepository interface {
	CreateMessage(message *models.ChatMessage) error
	GetConversationContext(conversationID, userID, domain string, limit int) ([]models.ChatMessage, error)
	GetConversationHistory(conversationID, userID, domain string) ([]models.ChatMessage, error)
}

type knowledgeAssistantPreferencesRepository interface {
	GetOrCreate(userID string) (*models.UserPreferences, error)
}

type knowledgeAssistantRepository interface {
	AssistantContext(userID, searchText string, topicLimit, entryLimit int) (*models.KnowledgeAssistantContext, error)
	CreateInboxFallback(ctx context.Context, userID, body, title string) (*models.KnowledgeEntry, bool, error)
	CreateEntry(userID string, entry *models.KnowledgeEntry) error
	Search(userID string, filter models.KnowledgeSearchFilter) ([]models.KnowledgeSearchResult, error)
	UpdateEntry(userID, entryID string, expectedVersion int, mutation models.KnowledgeEntryMutation, changedBy string) (*models.KnowledgeEntry, error)
	DeleteEntry(userID, entryID string, expectedVersion int, changedBy string) error
}

type knowledgeIntentDetector func(ctx context.Context, question string, conversationHistory []models.ChatMessage, knowledgeContext *models.KnowledgeAssistantContext, apiKey string) (*models.KnowledgeAssistantIntentResponse, error)
type knowledgeAnswerGenerator func(ctx context.Context, question string, results []models.KnowledgeSearchResult, conversationHistory []models.ChatMessage, apiKey, modelName string) (string, error)

func NewKnowledgeAssistantHandler(cfg *config.Config, chatRepo *repository.ChatRepository, prefsRepo *repository.PreferencesRepository, knowledgeRepo *repository.KnowledgeRepository, organizer KnowledgeOrganizer) *KnowledgeAssistantHandler {
	return &KnowledgeAssistantHandler{
		cfg:            cfg,
		chatRepo:       chatRepo,
		prefsRepo:      prefsRepo,
		knowledgeRepo:  knowledgeRepo,
		organizer:      organizer,
		detectIntent:   utils.DetectKnowledgeIntent,
		generateAnswer: utils.GenerateKnowledgeAnswer,
	}
}

func (h *KnowledgeAssistantHandler) AskQuestion(c echo.Context) error {
	userID := c.Get("userID").(string)
	var request models.AssistantRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	request.Question = strings.TrimSpace(request.Question)
	if request.Question == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "question is required")
	}
	if len([]rune(request.Question)) > 2000 {
		return echo.NewHTTPError(http.StatusBadRequest, "question must be 2000 characters or fewer")
	}

	conversationID, err := assistantConversationID(request.ConversationID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "conversationId must be a valid UUID")
	}
	history, err := h.chatRepo.GetConversationContext(conversationID, userID, models.ChatDomainKnowledge, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch conversation history")
	}

	assistantModel := models.DefaultAssistantModel
	preferences, preferencesErr := h.prefsRepo.GetOrCreate(userID)
	if preferencesErr == nil && preferences != nil {
		assistantModel = preferences.AssistantModel
	}
	if !models.IsSupportedGeminiModel(assistantModel) {
		assistantModel = models.DefaultAssistantModel
	}
	knowledgeContext, err := h.knowledgeRepo.AssistantContext(userID, request.Question, 50, 20)
	if err != nil {
		log.Printf("Failed to initialize knowledge assistant context for user %s: %v", userID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize personal knowledge")
	}

	intent, err := h.detectIntent(c.Request().Context(), request.Question, history, knowledgeContext, h.cfg.GeminiAPIKey)
	if err != nil {
		if requestCanceled(c.Request().Context(), err) {
			return echo.NewHTTPError(http.StatusRequestTimeout, "request was canceled before it could be classified")
		}
		if !plausiblyKnowledgeWriteRequest(request.Question) {
			log.Printf("Knowledge intent detection failed for user %s: %v", userID, err)
			return echo.NewHTTPError(http.StatusServiceUnavailable, "knowledge assistant request could not be classified; please retry")
		}
		log.Printf("Knowledge intent detection failed for user %s; preserving plausible write in Inbox: %v", userID, err)
		answer, fallbackErr := h.saveInboxFallback(c.Request().Context(), userID, request.Question)
		if fallbackErr != nil {
			return h.knowledgeSaveHTTPError(fallbackErr, "knowledge request could not be saved")
		}
		intent = &models.KnowledgeAssistantIntentResponse{Type: "direct", Answer: answer}
	}

	answer, err := h.executeKnowledgeIntent(c, userID, request.Question, intent, knowledgeContext, history, assistantModel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(answer) == "" {
		answer = "I couldn't produce a knowledge response. Please try again."
	}

	userMessage := &models.ChatMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Domain:         models.ChatDomainKnowledge,
		Role:           "user",
		Content:        request.Question,
	}
	if saveErr := h.chatRepo.CreateMessage(userMessage); saveErr != nil {
		log.Printf("Failed to save knowledge user message: %v", saveErr)
	}
	assistantMessage := &models.ChatMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Domain:         models.ChatDomainKnowledge,
		Role:           "assistant",
		Content:        answer,
	}
	if saveErr := h.chatRepo.CreateMessage(assistantMessage); saveErr != nil {
		log.Printf("Failed to save knowledge assistant message: %v", saveErr)
	}

	return c.JSON(http.StatusOK, models.AssistantResponse{
		Answer:         answer,
		ConversationID: conversationID,
	})
}

func (h *KnowledgeAssistantHandler) executeKnowledgeIntent(c echo.Context, userID, question string, intent *models.KnowledgeAssistantIntentResponse, knowledgeContext *models.KnowledgeAssistantContext, history []models.ChatMessage, assistantModel string) (string, error) {
	switch intent.Type {
	case "direct":
		return intent.Answer, nil
	case "knowledge_write":
		if plausiblyKnowledgeOrganizeRequest(question) {
			return "I understood this as an organize command, but I couldn't identify one exact topic safely. I did not save it as a note.", nil
		}
		if intent.Confidence != "high" || intent.Knowledge == nil || intent.Knowledge.Operation != "create" {
			if !plausiblyKnowledgeWriteRequest(question) {
				return "I wasn't confident that you wanted this saved. Please explicitly ask me to remember or note it; I did not create an Inbox entry.", nil
			}
			answer, err := h.saveInboxFallback(c.Request().Context(), userID, question)
			if err != nil {
				return "", h.knowledgeSaveHTTPError(err, "the request was ambiguous and could not be saved to Inbox")
			}
			return answer, nil
		}
		answer, err := h.createKnowledgeFromIntent(userID, question, intent.Knowledge, knowledgeContext)
		if err == nil {
			return answer, nil
		}
		if !plausiblyKnowledgeWriteRequest(question) {
			log.Printf("Structured knowledge write failed for user %s and was not eligible for Inbox fallback: %v", userID, err)
			return "I couldn't save that request safely, so I did not create an Inbox entry. Please explicitly ask me to remember the exact information.", nil
		}
		log.Printf("Structured knowledge write failed for user %s; preserving original text in Inbox: %v", userID, err)
		answer, fallbackErr := h.saveInboxFallback(c.Request().Context(), userID, question)
		if fallbackErr != nil {
			return "", h.knowledgeSaveHTTPError(fallbackErr, "knowledge could not be saved")
		}
		return answer, nil
	case "knowledge_query":
		answer, err := h.answerKnowledgeQuery(c, userID, question, intent, history, assistantModel)
		if err != nil {
			if requestCanceled(c.Request().Context(), err) {
				return "", echo.NewHTTPError(http.StatusRequestTimeout, "request was canceled before an answer could be generated")
			}
			return "", knowledgeHTTPError(err, "failed to search personal knowledge")
		}
		return answer, nil
	case "knowledge_change":
		answer, err := h.changeKnowledgeFromIntent(userID, intent, knowledgeContext)
		if err != nil {
			return "", knowledgeHTTPError(err, "failed to change personal knowledge")
		}
		return answer, nil
	case "knowledge_forget":
		answer, err := h.forgetKnowledgeFromIntent(userID, intent, knowledgeContext)
		if err != nil {
			return "", knowledgeHTTPError(err, "failed to forget personal knowledge")
		}
		return answer, nil
	case "knowledge_organize":
		answer, err := h.organizeKnowledgeFromIntent(c.Request().Context(), userID, intent, knowledgeContext)
		if err != nil {
			if requestCanceled(c.Request().Context(), err) {
				return "", echo.NewHTTPError(http.StatusRequestTimeout, "topic organization was interrupted; please try again")
			}
			log.Printf("Knowledge assistant topic organization failed for user %s: %v", userID, err)
			return "", echo.NewHTTPError(http.StatusBadGateway, "topic organization failed; please try again")
		}
		return answer, nil
	default:
		return "", echo.NewHTTPError(http.StatusInternalServerError, "unsupported knowledge assistant intent")
	}
}

func (h *KnowledgeAssistantHandler) knowledgeSaveHTTPError(err error, message string) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return echo.NewHTTPError(http.StatusRequestTimeout, "request was canceled before it could be saved")
	}
	log.Printf("Knowledge assistant save failed: %v", err)
	return echo.NewHTTPError(http.StatusInternalServerError, message)
}

func requestCanceled(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (h *KnowledgeAssistantHandler) GetConversationHistory(c echo.Context) error {
	userID := c.Get("userID").(string)
	conversationID, parseErr := assistantConversationID(c.Param("conversationId"))
	if parseErr != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "conversationId must be a valid UUID")
	}
	messages, err := h.chatRepo.GetConversationHistory(conversationID, userID, models.ChatDomainKnowledge)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch conversation history")
	}
	return c.JSON(http.StatusOK, messages)
}
