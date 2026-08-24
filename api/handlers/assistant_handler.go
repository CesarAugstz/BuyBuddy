package handlers

import (
	"buybuddy-api/config"
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"buybuddy-api/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type AssistantHandler struct {
	cfg           *config.Config
	receiptRepo   *repository.ReceiptRepository
	chatRepo      *repository.ChatRepository
	prefsRepo     *repository.PreferencesRepository
	categoryRepo  *repository.CategoryRepository
	knowledgeRepo *repository.KnowledgeRepository
	organizer     KnowledgeOrganizer
}

func NewAssistantHandler(cfg *config.Config, receiptRepo *repository.ReceiptRepository, chatRepo *repository.ChatRepository, prefsRepo *repository.PreferencesRepository, categoryRepo *repository.CategoryRepository, knowledgeRepo *repository.KnowledgeRepository, organizers ...KnowledgeOrganizer) *AssistantHandler {
	handler := &AssistantHandler{
		cfg:           cfg,
		receiptRepo:   receiptRepo,
		chatRepo:      chatRepo,
		prefsRepo:     prefsRepo,
		categoryRepo:  categoryRepo,
		knowledgeRepo: knowledgeRepo,
	}
	if len(organizers) > 0 {
		handler.organizer = organizers[0]
	}
	return handler
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

	originalQuestion := req.Question
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "question is required")
	}
	if len([]rune(req.Question)) > 2000 {
		return echo.NewHTTPError(http.StatusBadRequest, "question must be 2000 characters or fewer")
	}

	conversationID := req.ConversationID
	if conversationID == "" {
		conversationID = uuid.New().String()
	}

	conversationHistory, err := h.chatRepo.GetConversationContext(conversationID, userID, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch conversation history")
	}

	prefs, _ := h.prefsRepo.GetOrCreate(userID)
	assistantModel := prefs.AssistantModel
	if !models.IsSupportedGeminiModel(assistantModel) {
		assistantModel = models.DefaultAssistantModel
	}

	firstReceiptDate := h.getFirstReceiptDate(userID)

	categories, _ := h.categoryRepo.GetAll()

	knowledgeContext, err := h.knowledgeRepo.AssistantContext(userID, req.Question, 50, 20)
	if err != nil {
		log.Printf("Failed to initialize knowledge for user %s: %v", userID, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to initialize personal knowledge")
	}

	intent, err := utils.DetectIntentAndGenerateQuery(c.Request().Context(), req.Question, conversationHistory, firstReceiptDate, categories, h.cfg.GeminiAPIKey, knowledgeContext)
	if err != nil {
		if c.Request().Context().Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return echo.NewHTTPError(http.StatusRequestTimeout, "request was canceled before it could be classified")
		}
		if !plausiblyKnowledgeWriteRequest(originalQuestion) {
			log.Printf("Intent detection failed for user %s; request was not eligible for Inbox fallback: %v", userID, err)
			return echo.NewHTTPError(http.StatusServiceUnavailable, "assistant request could not be classified; please retry")
		}
		log.Printf("Intent detection failed for user %s; using knowledge-write Inbox fallback: %v", userID, err)
		answer, fallbackErr := h.saveInboxFallback(c.Request().Context(), userID, originalQuestion)
		if fallbackErr != nil {
			log.Printf("Failed to save Inbox fallback for user %s: %v", userID, fallbackErr)
			if errors.Is(fallbackErr, context.Canceled) || errors.Is(fallbackErr, context.DeadlineExceeded) {
				return echo.NewHTTPError(http.StatusRequestTimeout, "request was canceled before it could be saved")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "knowledge request could not be saved")
		}
		intent = &models.AssistantIntentResponse{Type: "direct", Answer: answer}
	}

	var answer string

	switch intent.Type {
	case "direct":
		answer = intent.Answer
	case "query", "receipt_query":
		compactReceipts, queryErr := h.queryReceipts(userID, intent)
		if queryErr != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to query receipt history")
		}
		answer, err = utils.GenerateAnswer(c.Request().Context(), req.Question, compactReceipts, conversationHistory, h.cfg.GeminiAPIKey, assistantModel)
		if err != nil {
			fmt.Println("Answer generation error:", err)
			return echo.NewHTTPError(http.StatusInternalServerError, map[string]string{
				"message": "Failed to get answer from assistant",
				"error":   err.Error(),
			})
		}
	case "knowledge_write":
		if plausiblyKnowledgeOrganizeRequest(originalQuestion) {
			answer = "I understood this as an organize command, but I couldn't identify one exact topic safely. I did not save it as a knowledge note."
			break
		}
		if intent.Confidence != "high" || intent.Knowledge == nil || intent.Knowledge.Operation != "create" {
			answer, err = h.saveInboxFallback(c.Request().Context(), userID, originalQuestion)
			if err != nil {
				log.Printf("Failed to save ambiguous write to Inbox for user %s: %v", userID, err)
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return echo.NewHTTPError(http.StatusRequestTimeout, "request was canceled before it could be saved")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "the request was ambiguous and could not be saved to Inbox")
			}
			break
		}
		answer, err = h.createKnowledgeFromIntent(userID, originalQuestion, intent.Knowledge, knowledgeContext)
		if err != nil {
			log.Printf("Structured knowledge write failed for user %s; using Inbox fallback: %v", userID, err)
			answer, err = h.saveInboxFallback(c.Request().Context(), userID, originalQuestion)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return echo.NewHTTPError(http.StatusRequestTimeout, "request was canceled before it could be saved")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "knowledge could not be saved")
			}
		}
	case "knowledge_query":
		answer, err = h.answerKnowledgeQuery(c, userID, req.Question, intent, conversationHistory, assistantModel, nil)
		if err != nil {
			return knowledgeHTTPError(err, "failed to search personal knowledge")
		}
	case "combined_query":
		answer, err = h.answerCombinedKnowledgeQuery(c, userID, req.Question, intent, conversationHistory, assistantModel)
		if err != nil {
			return knowledgeHTTPError(err, "failed to answer combined question")
		}
	case "knowledge_change":
		answer, err = h.changeKnowledgeFromIntent(userID, intent, knowledgeContext)
		if err != nil {
			return knowledgeHTTPError(err, "failed to change personal knowledge")
		}
	case "knowledge_forget":
		answer, err = h.forgetKnowledgeFromIntent(userID, intent, knowledgeContext)
		if err != nil {
			return knowledgeHTTPError(err, "failed to forget personal knowledge")
		}
	case "knowledge_organize":
		answer, err = h.organizeKnowledgeFromIntent(c.Request().Context(), userID, intent, knowledgeContext)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return echo.NewHTTPError(http.StatusRequestTimeout, "topic organization was interrupted; please try again")
			}
			log.Printf("Assistant topic organization failed for user %s: %v", userID, err)
			return echo.NewHTTPError(http.StatusBadGateway, "topic organization failed; please try again")
		}
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "unsupported assistant intent")
	}

	userMessage := &models.ChatMessage{
		ConversationID: conversationID,
		UserID:         userID,
		Role:           "user",
		Content:        req.Question,
	}
	if err := h.chatRepo.CreateMessage(userMessage); err != nil {
		fmt.Println("Failed to save user message:", err)
	} else {
		utils.GetAssistantSuggestionCache().Invalidate(userID)
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

func (h *AssistantHandler) queryReceipts(userID string, intent *models.AssistantIntentResponse) (*models.CompactReceiptResponse, error) {
	if intent.Specific == nil && intent.General == nil {
		if intent.Type == "combined_query" {
			return nil, nil
		}
		return utils.FormatReceiptsCompact(nil, nil), nil
	}

	var results []models.Receipt
	var err error
	effectiveFilter := intent.Specific

	log.Printf("Specific query filters: %+v", intent.Specific)
	log.Printf("General query filters: %+v", intent.General)
	if intent.Specific != nil {
		results, err = h.receiptRepo.QueryWithFilters(userID, intent.Specific, 30)
		if err != nil {
			return nil, err
		}
	}
	if len(results) == 0 && intent.General != nil {
		results, err = h.receiptRepo.QueryWithFilters(userID, intent.General, 30)
		if err != nil {
			return nil, err
		}
		effectiveFilter = intent.General
	}
	return utils.FormatReceiptsCompact(results, effectiveFilter), nil
}

func (h *AssistantHandler) saveInboxFallback(ctx context.Context, userID, originalText string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	body := strings.TrimSpace(originalText)
	entry, created, err := h.knowledgeRepo.CreateInboxFallback(ctx, userID, body, knowledgeFallbackTitle(body))
	if err != nil {
		return "", err
	}
	if !created {
		return fmt.Sprintf("That same request was already saved recently in **Inbox** as “%s”.", entry.Title), nil
	}
	log.Printf("Saved assistant knowledge-write fallback to Inbox for user %s as entry %s", userID, entry.ID)
	return fmt.Sprintf("I couldn't classify that confidently, so I saved the original text to **Inbox** as “%s”.", entry.Title), nil
}

func plausiblyKnowledgeWriteRequest(text string) bool {
	value := normalizeKnowledgeCommandText(text)
	if value == "" {
		return false
	}

	for _, prefix := range []string{
		"my preference is ", "my preference: ", "i prefer ", "set my preference to ",
		"dear diary", "diary: ",
		"minha preferência é ", "minha preferencia e ", "minha preferência: ", "minha preferencia: ",
		"eu prefiro ", "prefiro ", "defina minha preferência como ", "defina minha preferencia como ",
		"querido diário", "querido diario", "diário: ", "diario: ",
		"mi preferencia es ", "mi preferencia: ", "prefiero ",
	} {
		if strings.HasPrefix(value, prefix) && len(strings.TrimSpace(strings.TrimPrefix(value, prefix))) > 0 {
			return true
		}
	}

	questionWords := []string{
		"when", "where", "who", "why", "how", "what", "which", "can you", "could you",
		"quando", "onde", "quem", "por que", "porque", "como", "qual", "quanto", "você", "voce",
		"cuándo", "cuando", "dónde", "donde", "quién", "quien", "por qué", "como", "qué",
	}
	for _, command := range []string{"remember", "remember this", "lembre", "lembre-se", "lembra", "memorize", "recuerda"} {
		if rest, ok := knowledgeCommandRest(value, command); ok && !startsWithKnowledgePhrase(rest, questionWords) {
			return true
		}
	}

	for _, command := range []string{
		"save", "note", "write down", "record",
		"salve", "salvar", "guarde", "guardar", "anote", "anota", "anotar", "registre", "registra", "registrar",
		"guarda",
	} {
		rest, ok := knowledgeCommandRest(value, command)
		if !ok || startsWithKnowledgePhrase(rest, questionWords) {
			continue
		}
		if command == "save" && startsWithKnowledgePhrase(rest, []string{"money", "time", "on", "energy", "space"}) {
			continue
		}
		return true
	}

	for _, phrase := range []string{
		"add this to my notes", "add that to my notes", "add this to my diary", "add that to my diary",
		"adicione isto às minhas notas", "adicione isso às minhas notas", "adicione isto ao meu diário", "adicione isso ao meu diário",
		"adiciona isso nas minhas notas", "adiciona isso no meu diário",
	} {
		if strings.HasPrefix(value, phrase) {
			return true
		}
	}
	return false
}

func plausiblyKnowledgeOrganizeRequest(text string) bool {
	value := normalizeKnowledgeCommandText(text)
	for _, command := range []string{
		"organize", "organise", "clean up",
		"organiza", "organizar", "arrume", "arruma", "limpe", "limpar",
	} {
		if _, ok := knowledgeCommandRest(value, command); ok {
			return true
		}
	}
	return false
}

func normalizeKnowledgeCommandText(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > 512 {
		runes = runes[:512]
	}
	value := strings.ToLower(strings.Join(strings.Fields(string(runes)), " "))
	value = strings.Trim(value, " \t\r\n\"'“”")
	for _, prefix := range []string{
		"could you please ", "would you please ", "can you please ",
		"could you ", "would you ", "can you ", "please, ", "please ",
		"poderia por favor ", "pode por favor ", "poderia ", "pode ",
		"por favor, ", "por favor ", "favor, ", "favor ", "por favorzinho, ", "por favorzinho ",
	} {
		if strings.HasPrefix(value, prefix) {
			value = strings.TrimSpace(strings.TrimPrefix(value, prefix))
			break
		}
	}
	if value == "" {
		return ""
	}
	return value
}

func knowledgeCommandRest(value, command string) (string, bool) {
	for _, separator := range []string{" ", ": "} {
		prefix := command + separator
		if strings.HasPrefix(value, prefix) {
			rest := strings.TrimSpace(strings.TrimPrefix(value, prefix))
			return rest, rest != ""
		}
	}
	return "", false
}

func startsWithKnowledgePhrase(value string, phrases []string) bool {
	for _, phrase := range phrases {
		if value == phrase || strings.HasPrefix(value, phrase+" ") || strings.HasPrefix(value, phrase+"?") {
			return true
		}
	}
	return false
}

func knowledgeFallbackTitle(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	runes := []rune(text)
	if len(runes) > 80 {
		text = string(runes[:80]) + "…"
	}
	if text == "" {
		return "Saved note"
	}
	return text
}

func (h *AssistantHandler) createKnowledgeFromIntent(userID, originalText string, action *models.AssistantKnowledgeAction, context *models.KnowledgeAssistantContext) (string, error) {
	topicID := strings.TrimSpace(action.TopicID)
	if topicID == "" || !assistantContextHasTopic(context, topicID) {
		return "", repository.ErrKnowledgeInvalid
	}
	kind := strings.TrimSpace(action.Kind)
	if kind == "" {
		kind = "note"
	}
	title := strings.TrimSpace(action.Title)
	if title == "" {
		title = knowledgeFallbackTitle(originalText)
	}
	var occurredAt *time.Time
	if action.OccurredAt != "" {
		parsed, err := time.Parse(time.RFC3339, action.OccurredAt)
		if err != nil {
			return "", fmt.Errorf("%w: invalid occurredAt", repository.ErrKnowledgeInvalid)
		}
		occurredAt = &parsed
	}
	entry := &models.KnowledgeEntry{
		TopicID:    topicID,
		Kind:       kind,
		Title:      title,
		Body:       strings.TrimSpace(originalText),
		Attributes: action.Attributes,
		Tags:       action.Tags,
		OccurredAt: occurredAt,
		Source:     models.KnowledgeSourceAssistant,
	}
	if err := h.knowledgeRepo.CreateEntry(userID, entry); err != nil {
		return "", err
	}
	topicPath := assistantTopicPath(context, topicID)
	return fmt.Sprintf("Saved **%s** in **%s**.", entry.Title, topicPath), nil
}

func (h *AssistantHandler) answerKnowledgeQuery(c echo.Context, userID, question string, intent *models.AssistantIntentResponse, conversationHistory []models.ChatMessage, assistantModel string, receipts *models.CompactReceiptResponse) (string, error) {
	results, err := h.searchKnowledgeForIntent(userID, question, intent)
	if err != nil {
		return "", err
	}
	return h.generateKnowledgeAnswer(c, userID, question, results, receipts, conversationHistory, assistantModel)
}

func (h *AssistantHandler) answerCombinedKnowledgeQuery(c echo.Context, userID, question string, intent *models.AssistantIntentResponse, conversationHistory []models.ChatMessage, assistantModel string) (string, error) {
	results, err := h.searchKnowledgeForIntent(userID, question, intent)
	if err != nil {
		return "", err
	}
	enrichedIntent := EnrichReceiptFiltersFromKnowledge(intent, results)
	receipts, err := h.queryReceipts(userID, enrichedIntent)
	if err != nil {
		return "", err
	}
	return h.generateKnowledgeAnswer(c, userID, question, results, receipts, conversationHistory, assistantModel)
}

func (h *AssistantHandler) searchKnowledgeForIntent(userID, question string, intent *models.AssistantIntentResponse) ([]models.KnowledgeSearchResult, error) {
	searchQuery := question
	if intent.Knowledge != nil && strings.TrimSpace(intent.Knowledge.SearchQuery) != "" {
		searchQuery = intent.Knowledge.SearchQuery
	}
	searchQuery = truncateAssistantText(searchQuery, 500)
	return h.knowledgeRepo.Search(userID, models.KnowledgeSearchFilter{
		Query: searchQuery,
		Limit: 20,
	})
}

func (h *AssistantHandler) generateKnowledgeAnswer(c echo.Context, userID, question string, results []models.KnowledgeSearchResult, receipts *models.CompactReceiptResponse, conversationHistory []models.ChatMessage, assistantModel string) (string, error) {
	answer, err := utils.GenerateKnowledgeAnswer(c.Request().Context(), question, results, receipts, conversationHistory, h.cfg.GeminiAPIKey, assistantModel)
	if err != nil {
		log.Printf("Knowledge answer generation failed for user %s: %v", userID, err)
		return formatKnowledgeResults(results, receipts), nil
	}
	return answer, nil
}

func EnrichReceiptFiltersFromKnowledge(intent *models.AssistantIntentResponse, results []models.KnowledgeSearchResult) *models.AssistantIntentResponse {
	if intent == nil || intent.Type != "combined_query" {
		return intent
	}
	enriched := *intent
	enriched.Specific = cloneAssistantQueryFilter(intent.Specific)
	enriched.General = cloneAssistantQueryFilter(intent.General)

	products, brands := knowledgeProductAttributes(results)
	enrich := func(filter *models.AssistantQueryFilter) {
		if filter == nil {
			return
		}
		if len(filter.ProductName) == 0 {
			filter.ProductName = append([]string(nil), products...)
		}
		if len(filter.Brand) == 0 {
			filter.Brand = append([]string(nil), brands...)
		}
	}
	enrich(enriched.Specific)
	enrich(enriched.General)
	if enriched.Specific == nil && (len(products) > 0 || len(brands) > 0) {
		enriched.Specific = &models.AssistantQueryFilter{
			ProductName:  append([]string(nil), products...),
			Brand:        append([]string(nil), brands...),
			ProductScope: "specific",
		}
	}
	if enriched.General == nil && enriched.Specific != nil {
		enriched.General = cloneAssistantQueryFilter(enriched.Specific)
	}
	return &enriched
}

func cloneAssistantQueryFilter(filter *models.AssistantQueryFilter) *models.AssistantQueryFilter {
	if filter == nil {
		return nil
	}
	cloned := *filter
	cloned.ProductName = append([]string(nil), filter.ProductName...)
	cloned.Company = append([]string(nil), filter.Company...)
	cloned.Brand = append([]string(nil), filter.Brand...)
	cloned.Category = append([]string(nil), filter.Category...)
	cloned.Subcategory = append([]string(nil), filter.Subcategory...)
	return &cloned
}

func knowledgeProductAttributes(results []models.KnowledgeSearchResult) ([]string, []string) {
	products := make([]string, 0, 10)
	brands := make([]string, 0, 10)
	add := func(values *[]string, value string) {
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) > 120 || len(*values) == 10 {
			return
		}
		for _, existing := range *values {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		*values = append(*values, value)
	}
	addValue := func(values *[]string, raw interface{}) {
		switch value := raw.(type) {
		case string:
			add(values, value)
		case []string:
			for _, item := range value {
				add(values, item)
			}
		case []interface{}:
			for _, item := range value {
				if text, ok := item.(string); ok {
					add(values, text)
				}
			}
		}
	}
	for index, result := range results {
		if index == 10 {
			break
		}
		for key, value := range result.Entry.Attributes {
			normalizedKey := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
			switch normalizedKey {
			case "product", "productname":
				addValue(&products, value)
			case "brand":
				addValue(&brands, value)
			}
		}
	}
	return products, brands
}

func (h *AssistantHandler) changeKnowledgeFromIntent(userID string, intent *models.AssistantIntentResponse, context *models.KnowledgeAssistantContext) (string, error) {
	action, entry, ok := exactAssistantEntry(intent, context)
	if !ok || action.Operation != "update" {
		return "I found no single, unambiguous entry to change. Please name the entry more precisely.", nil
	}
	mutation := models.KnowledgeEntryMutation{}
	if action.TopicID != "" {
		if !assistantContextHasTopic(context, action.TopicID) {
			return "I couldn't verify the requested destination topic, so I did not change anything.", nil
		}
		value := action.TopicID
		mutation.TopicID = &value
	}
	if action.Kind != "" {
		value := action.Kind
		mutation.Kind = &value
	}
	if action.Title != "" {
		value := action.Title
		mutation.Title = &value
	}
	if action.Body != "" {
		value := action.Body
		mutation.Body = &value
	}
	if action.Attributes != nil {
		value := action.Attributes
		mutation.Attributes = &value
	}
	if action.Tags != nil {
		value := action.Tags
		mutation.Tags = &value
	}
	if action.OccurredAt != "" {
		value, err := time.Parse(time.RFC3339, action.OccurredAt)
		if err != nil {
			return "I couldn't understand the requested occurrence date, so I did not change anything.", nil
		}
		mutation.OccurredAt = &value
	}
	if mutation.TopicID == nil && mutation.Kind == nil && mutation.Title == nil && mutation.Body == nil && mutation.Attributes == nil && mutation.Tags == nil && mutation.OccurredAt == nil {
		return "I understood which entry you meant, but not what should change. I did not modify it.", nil
	}
	updated, err := h.knowledgeRepo.UpdateEntry(userID, entry.ID, entry.Version, mutation, models.KnowledgeSourceAssistant)
	if err != nil {
		if errors.Is(err, repository.ErrKnowledgeConflict) {
			return "That entry changed before I could update it. Please try again.", nil
		}
		return "", err
	}
	return fmt.Sprintf("Updated **%s**. Its previous version is available through undo.", updated.Title), nil
}

func (h *AssistantHandler) forgetKnowledgeFromIntent(userID string, intent *models.AssistantIntentResponse, context *models.KnowledgeAssistantContext) (string, error) {
	action, entry, ok := exactAssistantEntry(intent, context)
	if !ok || action.Operation != "delete" {
		return "I found no single, unambiguous entry to forget. Please name the entry more precisely; I did not delete anything.", nil
	}
	if err := h.knowledgeRepo.DeleteEntry(userID, entry.ID, entry.Version, models.KnowledgeSourceAssistant); err != nil {
		if errors.Is(err, repository.ErrKnowledgeConflict) {
			return "That entry changed before I could forget it. Please try again.", nil
		}
		return "", err
	}
	return fmt.Sprintf("Forgot **%s**. It was soft-deleted and can be restored with undo.", entry.Title), nil
}

func (h *AssistantHandler) organizeKnowledgeFromIntent(ctx context.Context, userID string, intent *models.AssistantIntentResponse, knowledgeContext *models.KnowledgeAssistantContext) (string, error) {
	action, topic, ok := exactAssistantTopic(intent, knowledgeContext)
	if !ok || action.Operation != "organize" {
		return "I couldn't identify one exact topic to organize. Please name a topic from Personal Knowledge more precisely.", nil
	}
	if h.organizer == nil {
		return "Topic organization is unavailable right now. Please try again later.", nil
	}

	response, err := h.organizer.OrganizeTopic(ctx, userID, topic.ID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrKnowledgeConflict):
			return fmt.Sprintf("**%s** is already being organized. Please try again shortly.", topic.Path), nil
		case errors.Is(err, repository.ErrKnowledgeNotFound):
			return "That topic is no longer available, so I did not organize anything.", nil
		case errors.Is(err, repository.ErrKnowledgeInvalid):
			return "I couldn't safely organize that topic. No knowledge note was created.", nil
		default:
			return "", err
		}
	}
	if response == nil {
		return "", errors.New("organizer returned an empty response")
	}
	if response.Result.OperationsApplied == 0 {
		return fmt.Sprintf("Organized **%s**. No changes were needed.", topic.Path), nil
	}
	return fmt.Sprintf("Organized **%s** and applied %d changes.", topic.Path, response.Result.OperationsApplied), nil
}

func exactAssistantTopic(intent *models.AssistantIntentResponse, context *models.KnowledgeAssistantContext) (*models.AssistantKnowledgeAction, *models.KnowledgeAssistantTopic, bool) {
	if intent == nil || intent.Type != "knowledge_organize" || intent.Confidence != "high" || intent.Knowledge == nil || context == nil {
		return nil, nil, false
	}
	action := intent.Knowledge
	for index := range context.Topics {
		topic := &context.Topics[index]
		if topic.ID == action.TopicID {
			return action, topic, true
		}
	}
	return action, nil, false
}

func exactAssistantEntry(intent *models.AssistantIntentResponse, context *models.KnowledgeAssistantContext) (*models.AssistantKnowledgeAction, *models.KnowledgeAssistantEntry, bool) {
	if intent == nil || intent.Confidence != "high" || intent.Knowledge == nil || context == nil {
		return nil, nil, false
	}
	action := intent.Knowledge
	for i := range context.Entries {
		entry := &context.Entries[i]
		if entry.ID == action.EntryID && action.ExpectedVersion == entry.Version {
			return action, entry, true
		}
	}
	return action, nil, false
}

func assistantContextHasTopic(context *models.KnowledgeAssistantContext, topicID string) bool {
	for _, topic := range context.Topics {
		if topic.ID == topicID {
			return true
		}
	}
	return false
}

func assistantTopicPath(context *models.KnowledgeAssistantContext, topicID string) string {
	for _, topic := range context.Topics {
		if topic.ID == topicID {
			return topic.Path
		}
	}
	return models.KnowledgeInboxName
}

func truncateAssistantText(value string, maximum int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maximum {
		return string(runes)
	}
	return string(runes[:maximum])
}

func formatKnowledgeResults(results []models.KnowledgeSearchResult, receipts *models.CompactReceiptResponse) string {
	if len(results) == 0 {
		if receipts == nil {
			return "I found no matching personal knowledge."
		}
		if len(receipts.Receipts) == 0 {
			return "I found no matching personal knowledge or receipts."
		}
		return "I found no matching personal knowledge. Receipt history was available, but I couldn't generate a combined summary."
	}
	var builder strings.Builder
	builder.WriteString("I found:\n")
	for i, result := range results {
		if i >= 5 {
			break
		}
		builder.WriteString("- **")
		builder.WriteString(result.Entry.Title)
		builder.WriteString("**: ")
		builder.WriteString(truncateAssistantText(result.Entry.Body, 240))
		builder.WriteString("\n")
	}
	if receipts != nil && len(receipts.Receipts) == 0 {
		builder.WriteString("\nNo matching receipts were found.")
	} else if receipts != nil {
		builder.WriteString("\nI couldn't generate the combined receipt summary, so no receipt details were inferred.")
	}
	return strings.TrimSpace(builder.String())
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

func (h *AssistantHandler) GetSuggestions(c echo.Context) error {
	userID := c.Get("userID").(string)
	suggestionCache := utils.GetAssistantSuggestionCache()
	if suggestions, ok := suggestionCache.Get(userID); ok {
		return c.JSON(http.StatusOK, models.AssistantSuggestionsResponse{
			Suggestions: suggestions,
		})
	}

	messages, err := h.chatRepo.GetRecentUserQuestions(userID, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch recent questions")
	}

	questions := make([]string, 0, len(messages))
	for _, message := range messages {
		if question := strings.TrimSpace(message.Content); question != "" {
			questions = append(questions, question)
		}
	}

	suggestions, err := utils.GenerateQuestionSuggestions(
		c.Request().Context(),
		questions,
		h.cfg.GeminiAPIKey,
		models.DefaultAssistantModel,
	)
	if err != nil {
		log.Printf("Failed to generate assistant suggestions for user %s: %v", userID, err)
		suggestions = utils.DefaultQuestionSuggestions(questions)
	}
	suggestionCache.Set(userID, suggestions, 6*time.Hour)

	return c.JSON(http.StatusOK, models.AssistantSuggestionsResponse{
		Suggestions: suggestions,
	})
}
