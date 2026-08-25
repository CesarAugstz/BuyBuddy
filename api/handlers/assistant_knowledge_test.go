package handlers

import (
	"buybuddy-api/config"
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestAssistantHandlersHaveOneWayDomainDependencies(t *testing.T) {
	receiptHandler := reflect.TypeOf(AssistantHandler{})
	if _, exists := receiptHandler.FieldByName("knowledgeRepo"); exists {
		t.Fatal("receipt assistant can access the knowledge repository")
	}
	knowledgeHandler := reflect.TypeOf(KnowledgeAssistantHandler{})
	if _, exists := knowledgeHandler.FieldByName("receiptRepo"); exists {
		t.Fatal("knowledge assistant can access the receipt repository")
	}
}

func TestExactAssistantEntryRequiresHighConfidenceAndMatchingVersion(t *testing.T) {
	context := &models.KnowledgeAssistantContext{
		Entries: []models.KnowledgeAssistantEntry{{
			ID:      "entry-1",
			Title:   "Preferred milk",
			Version: 3,
		}},
	}
	intent := &models.KnowledgeAssistantIntentResponse{
		Type:       "knowledge_change",
		Confidence: "high",
		Knowledge: &models.AssistantKnowledgeAction{
			Operation:       "update",
			EntryID:         "entry-1",
			ExpectedVersion: 3,
		},
	}
	_, entry, ok := exactAssistantEntry(intent, context)
	if !ok || entry.ID != "entry-1" {
		t.Fatalf("exactAssistantEntry() = %#v, %t; want entry-1", entry, ok)
	}

	intent.Knowledge.ExpectedVersion = 2
	if _, _, ok := exactAssistantEntry(intent, context); ok {
		t.Fatal("stale version unexpectedly accepted")
	}
	intent.Knowledge.ExpectedVersion = 3
	intent.Confidence = "medium"
	if _, _, ok := exactAssistantEntry(intent, context); ok {
		t.Fatal("ambiguous mutation unexpectedly accepted")
	}
}

func TestKnowledgeFallbackTitleIsBounded(t *testing.T) {
	got := knowledgeFallbackTitle("  remember   this ")
	if got != "remember this" {
		t.Fatalf("knowledgeFallbackTitle() = %q", got)
	}
	long := make([]rune, 100)
	for i := range long {
		long[i] = 'a'
	}
	if got := []rune(knowledgeFallbackTitle(string(long))); len(got) != 81 || got[80] != '…' {
		t.Fatalf("bounded title length = %d, want 81 including ellipsis", len(got))
	}
}

func TestPlausiblyKnowledgeWriteRequestIsConservativeAndMultilingual(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{text: "Remember that I prefer oat milk", want: true},
		{text: "Could you please remember that pickup is best?", want: true},
		{text: "Please save this as a note", want: true},
		{text: "My preference is aisle pickup", want: true},
		{text: "Por favor, lembre que prefiro leite integral", want: true},
		{text: "Pode anotar que compro frutas aos domingos?", want: true},
		{text: "Anote isto no meu diário", want: true},
		{text: "Eu prefiro comprar frutas aos domingos", want: true},
		{text: "What did I spend on milk?", want: false},
		{text: "Remember when I bought milk?", want: false},
		{text: "Como economizar dinheiro?", want: false},
		{text: "Save money on groceries", want: false},
		{text: "Mostre meus recibos de ontem", want: false},
		{text: "Organize my diary", want: false},
		{text: "Clean up my Inbox", want: false},
	}
	for _, test := range tests {
		if got := plausiblyKnowledgeWriteRequest(test.text); got != test.want {
			t.Errorf("plausiblyKnowledgeWriteRequest(%q) = %t, want %t", test.text, got, test.want)
		}
	}
}

func TestPlausiblyKnowledgeOrganizeRequestPreventsNoteFallback(t *testing.T) {
	for _, command := range []string{
		"Organize my diary",
		"Please clean up my Inbox",
		"Organize minhas recomendações",
	} {
		if !plausiblyKnowledgeOrganizeRequest(command) {
			t.Errorf("plausiblyKnowledgeOrganizeRequest(%q) = false", command)
		}
		if plausiblyKnowledgeWriteRequest(command) {
			t.Errorf("organize command %q was eligible for knowledge-note fallback", command)
		}
	}
	if plausiblyKnowledgeOrganizeRequest("Remember to organize my diary tomorrow") {
		t.Fatal("a reminder containing organize was treated as an organize command")
	}
}

func TestOrganizeKnowledgeFromIntentUsesAuthenticatedUserAndExactContextTopic(t *testing.T) {
	runner := &fakeKnowledgeOrganizer{response: &models.KnowledgeOrganizationResponse{
		Result: models.KnowledgeOrganizationApplyResult{OperationsApplied: 2},
	}}
	handler := NewKnowledgeAssistantHandler(nil, nil, nil, nil, runner)
	knowledgeContext := &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Projects / BuyBuddy"}},
	}
	intent := &models.KnowledgeAssistantIntentResponse{
		Type:       "knowledge_organize",
		Confidence: "high",
		Knowledge: &models.AssistantKnowledgeAction{
			Operation: "organize",
			TopicID:   "topic-1",
		},
	}

	answer, err := handler.organizeKnowledgeFromIntent(context.Background(), "authenticated-user", intent, knowledgeContext)
	if err != nil {
		t.Fatalf("organizeKnowledgeFromIntent() error = %v", err)
	}
	if runner.userID != "authenticated-user" || runner.topicID != "topic-1" {
		t.Fatalf("organizer user/topic = %q/%q, want authenticated-user/topic-1", runner.userID, runner.topicID)
	}
	if !strings.Contains(answer, "Projects / BuyBuddy") || !strings.Contains(answer, "2 changes") {
		t.Fatalf("success answer = %q", answer)
	}
}

func TestOrganizeKnowledgeFromIntentRequiresHighConfidenceExactTopic(t *testing.T) {
	runner := &fakeKnowledgeOrganizer{}
	handler := &KnowledgeAssistantHandler{organizer: runner}
	knowledgeContext := &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Inbox"}},
	}
	intent := &models.KnowledgeAssistantIntentResponse{
		Type:       "knowledge_organize",
		Confidence: "medium",
		Knowledge:  &models.AssistantKnowledgeAction{Operation: "organize", TopicID: "topic-1"},
	}

	answer, err := handler.organizeKnowledgeFromIntent(context.Background(), "user", intent, knowledgeContext)
	if err != nil || !strings.Contains(answer, "one exact topic") {
		t.Fatalf("ambiguous answer/error = %q/%v", answer, err)
	}
	if runner.topicID != "" {
		t.Fatalf("ambiguous request invoked organizer for %q", runner.topicID)
	}

	intent.Confidence = "high"
	intent.Knowledge.TopicID = "topic-outside-context"
	answer, err = handler.organizeKnowledgeFromIntent(context.Background(), "user", intent, knowledgeContext)
	if err != nil || !strings.Contains(answer, "one exact topic") {
		t.Fatalf("out-of-context answer/error = %q/%v", answer, err)
	}
	if runner.topicID != "" {
		t.Fatalf("out-of-context request invoked organizer for %q", runner.topicID)
	}
}

func TestOrganizeKnowledgeFromIntentReportsNoChangeAndSafeLeaseConflict(t *testing.T) {
	knowledgeContext := &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Inbox"}},
	}
	intent := &models.KnowledgeAssistantIntentResponse{
		Type:       "knowledge_organize",
		Confidence: "high",
		Knowledge:  &models.AssistantKnowledgeAction{Operation: "organize", TopicID: "topic-1"},
	}
	handler := &KnowledgeAssistantHandler{organizer: &fakeKnowledgeOrganizer{
		response: &models.KnowledgeOrganizationResponse{},
	}}
	answer, err := handler.organizeKnowledgeFromIntent(context.Background(), "user", intent, knowledgeContext)
	if err != nil || !strings.Contains(answer, "No changes were needed") {
		t.Fatalf("no-change answer/error = %q/%v", answer, err)
	}

	handler.organizer = &fakeKnowledgeOrganizer{
		err: fmt.Errorf("%w: secret lease token", repository.ErrKnowledgeConflict),
	}
	answer, err = handler.organizeKnowledgeFromIntent(context.Background(), "user", intent, knowledgeContext)
	if err != nil || !strings.Contains(answer, "already being organized") || strings.Contains(answer, "secret lease token") {
		t.Fatalf("conflict answer/error = %q/%v", answer, err)
	}
}

func TestSaveInboxFallbackRejectsCanceledContextBeforeRepositoryUse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handler := &KnowledgeAssistantHandler{}
	if _, err := handler.saveInboxFallback(ctx, "user", "remember this"); !errors.Is(err, context.Canceled) {
		t.Fatalf("saveInboxFallback() error = %v, want context.Canceled", err)
	}
}

func TestQueryReceiptsWithoutFiltersReturnsError(t *testing.T) {
	handler := &AssistantHandler{}
	receipts, err := handler.queryReceipts("user", &models.ReceiptAssistantIntentResponse{Type: "receipt_query"})
	if err == nil {
		t.Fatalf("queryReceipts() = %#v, want missing-filter error", receipts)
	}
}

func TestReceiptDirectIntentEmptyAnswerGetsFallbackBeforePersistence(t *testing.T) {
	for _, answer := range []string{"", " \t\n"} {
		got := receiptAssistantAnswerOrFallback(answer)
		if strings.TrimSpace(got) == "" {
			t.Fatalf("receiptAssistantAnswerOrFallback(%q) returned an empty answer", answer)
		}
		if got != "I couldn't produce a receipt response. Please try again." {
			t.Fatalf("receiptAssistantAnswerOrFallback(%q) = %q", answer, got)
		}
	}
}

func TestFormatKnowledgeResultsNeverMentionsReceiptContext(t *testing.T) {
	result := models.KnowledgeSearchResult{
		Entry: models.KnowledgeEntry{Title: "Milk preference", Body: "Prefers whole milk."},
	}

	if got := formatKnowledgeResults(nil); strings.Contains(strings.ToLower(got), "receipt") {
		t.Fatalf("not-queried fallback unexpectedly mentions receipts: %q", got)
	}
	if got := formatKnowledgeResults([]models.KnowledgeSearchResult{result}); strings.Contains(strings.ToLower(got), "receipt") {
		t.Fatalf("knowledge result unexpectedly mentions receipts: %q", got)
	}
}

func TestKnowledgeAssistantEndpointDispatchesAllKnowledgeOperationsWithoutReceiptAccess(t *testing.T) {
	intents := []*models.KnowledgeAssistantIntentResponse{
		{
			Type:       "knowledge_write",
			Confidence: "high",
			Knowledge: &models.AssistantKnowledgeAction{
				Operation: "create",
				TopicID:   "topic-1",
				Kind:      "recommendation",
				Title:     "Favorite cafe",
			},
		},
		{
			Type:       "knowledge_query",
			Confidence: "high",
			Knowledge:  &models.AssistantKnowledgeAction{Operation: "search", SearchQuery: "cafe"},
		},
		{
			Type:       "knowledge_change",
			Confidence: "high",
			Knowledge: &models.AssistantKnowledgeAction{
				Operation:       "update",
				EntryID:         "entry-1",
				ExpectedVersion: 1,
				Title:           "Updated cafe",
			},
		},
		{
			Type:       "knowledge_forget",
			Confidence: "high",
			Knowledge: &models.AssistantKnowledgeAction{
				Operation:       "delete",
				EntryID:         "entry-1",
				ExpectedVersion: 1,
			},
		},
		{
			Type:       "knowledge_organize",
			Confidence: "high",
			Knowledge:  &models.AssistantKnowledgeAction{Operation: "organize", TopicID: "topic-1"},
		},
	}

	for _, intent := range intents {
		t.Run(intent.Type, func(t *testing.T) {
			chat := &fakeKnowledgeAssistantChatRepository{}
			knowledge := newFakeKnowledgeAssistantRepository()
			organizer := &fakeKnowledgeOrganizer{response: &models.KnowledgeOrganizationResponse{
				Result: models.KnowledgeOrganizationApplyResult{OperationsApplied: 1},
			}}
			handler := &KnowledgeAssistantHandler{
				cfg:           nil,
				chatRepo:      chat,
				prefsRepo:     fakeKnowledgeAssistantPreferencesRepository{},
				knowledgeRepo: knowledge,
				organizer:     organizer,
				detectIntent: func(context.Context, string, []models.ChatMessage, *models.KnowledgeAssistantContext, string) (*models.KnowledgeAssistantIntentResponse, error) {
					return intent, nil
				},
				generateAnswer: func(context.Context, string, []models.KnowledgeSearchResult, []models.ChatMessage, string, string) (string, error) {
					return "Found the saved cafe.", nil
				},
			}
			handler.cfg = &config.Config{}

			echoServer := echo.New()
			request := httptest.NewRequest(http.MethodPost, "/api/knowledge/assistant/ask", strings.NewReader(`{"question":"knowledge request"}`))
			request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			recorder := httptest.NewRecorder()
			echoContext := echoServer.NewContext(request, recorder)
			echoContext.Set("userID", "authenticated-user")

			if err := handler.AskQuestion(echoContext); err != nil {
				t.Fatalf("AskQuestion() error = %v", err)
			}
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			var response models.AssistantResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.ConversationID == "" {
				t.Fatalf("response = %#v, error = %v", response, err)
			}
			if len(chat.messages) != 2 {
				t.Fatalf("saved messages = %d, want 2", len(chat.messages))
			}
			for _, message := range chat.messages {
				if message.Domain != models.ChatDomainKnowledge {
					t.Fatalf("saved message domain = %q, want knowledge", message.Domain)
				}
			}

			switch intent.Type {
			case "knowledge_write":
				if knowledge.created == nil {
					t.Fatal("create intent did not create knowledge")
				}
			case "knowledge_query":
				if knowledge.searchQuery != "cafe" {
					t.Fatalf("search query = %q, want cafe", knowledge.searchQuery)
				}
			case "knowledge_change":
				if knowledge.updatedEntryID != "entry-1" {
					t.Fatal("change intent did not update exact entry")
				}
			case "knowledge_forget":
				if knowledge.deletedEntryID != "entry-1" {
					t.Fatal("forget intent did not delete exact entry")
				}
			case "knowledge_organize":
				if organizer.userID != "authenticated-user" || organizer.topicID != "topic-1" {
					t.Fatalf("organizer received %q/%q", organizer.userID, organizer.topicID)
				}
			}
		})
	}
}

type fakeKnowledgeAssistantChatRepository struct {
	messages []models.ChatMessage
}

func (f *fakeKnowledgeAssistantChatRepository) CreateMessage(message *models.ChatMessage) error {
	f.messages = append(f.messages, *message)
	return nil
}

func (f *fakeKnowledgeAssistantChatRepository) GetConversationContext(_, _, domain string, _ int) ([]models.ChatMessage, error) {
	if domain != models.ChatDomainKnowledge {
		return nil, fmt.Errorf("unexpected conversation domain %q", domain)
	}
	return nil, nil
}

func (f *fakeKnowledgeAssistantChatRepository) GetConversationHistory(_, _, domain string) ([]models.ChatMessage, error) {
	if domain != models.ChatDomainKnowledge {
		return nil, fmt.Errorf("unexpected conversation domain %q", domain)
	}
	return append([]models.ChatMessage(nil), f.messages...), nil
}

type fakeKnowledgeAssistantPreferencesRepository struct{}

func (fakeKnowledgeAssistantPreferencesRepository) GetOrCreate(string) (*models.UserPreferences, error) {
	return &models.UserPreferences{AssistantModel: models.DefaultAssistantModel}, nil
}

type fakeKnowledgeAssistantRepository struct {
	context        *models.KnowledgeAssistantContext
	created        *models.KnowledgeEntry
	searchQuery    string
	updatedEntryID string
	deletedEntryID string
}

func newFakeKnowledgeAssistantRepository() *fakeKnowledgeAssistantRepository {
	return &fakeKnowledgeAssistantRepository{context: &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Recommendations"}},
		Entries: []models.KnowledgeAssistantEntry{{
			ID:      "entry-1",
			TopicID: "topic-1",
			Title:   "Favorite cafe",
			Version: 1,
		}},
	}}
}

func (f *fakeKnowledgeAssistantRepository) AssistantContext(string, string, int, int) (*models.KnowledgeAssistantContext, error) {
	return f.context, nil
}

func (f *fakeKnowledgeAssistantRepository) CreateInboxFallback(_ context.Context, _, body, title string) (*models.KnowledgeEntry, bool, error) {
	entry := &models.KnowledgeEntry{ID: "fallback", TopicID: "topic-1", Title: title, Body: body}
	f.created = entry
	return entry, true, nil
}

func (f *fakeKnowledgeAssistantRepository) CreateEntry(_ string, entry *models.KnowledgeEntry) error {
	f.created = entry
	return nil
}

func (f *fakeKnowledgeAssistantRepository) Search(_ string, filter models.KnowledgeSearchFilter) ([]models.KnowledgeSearchResult, error) {
	f.searchQuery = filter.Query
	return []models.KnowledgeSearchResult{{Entry: models.KnowledgeEntry{Title: "Favorite cafe", Body: "Cafe A"}}}, nil
}

func (f *fakeKnowledgeAssistantRepository) UpdateEntry(_ string, entryID string, _ int, mutation models.KnowledgeEntryMutation, _ string) (*models.KnowledgeEntry, error) {
	f.updatedEntryID = entryID
	title := "Favorite cafe"
	if mutation.Title != nil {
		title = *mutation.Title
	}
	return &models.KnowledgeEntry{ID: entryID, Title: title}, nil
}

func (f *fakeKnowledgeAssistantRepository) DeleteEntry(_ string, entryID string, _ int, _ string) error {
	f.deletedEntryID = entryID
	return nil
}
