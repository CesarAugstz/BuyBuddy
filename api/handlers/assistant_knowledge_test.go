package handlers

import (
	"buybuddy-api/models"
	"buybuddy-api/repository"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestExactAssistantEntryRequiresHighConfidenceAndMatchingVersion(t *testing.T) {
	context := &models.KnowledgeAssistantContext{
		Entries: []models.KnowledgeAssistantEntry{{
			ID:      "entry-1",
			Title:   "Preferred milk",
			Version: 3,
		}},
	}
	intent := &models.AssistantIntentResponse{
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
	handler := NewAssistantHandler(nil, nil, nil, nil, nil, nil, runner)
	knowledgeContext := &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Projects / BuyBuddy"}},
	}
	intent := &models.AssistantIntentResponse{
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
	handler := &AssistantHandler{organizer: runner}
	knowledgeContext := &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Inbox"}},
	}
	intent := &models.AssistantIntentResponse{
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
	intent := &models.AssistantIntentResponse{
		Type:       "knowledge_organize",
		Confidence: "high",
		Knowledge:  &models.AssistantKnowledgeAction{Operation: "organize", TopicID: "topic-1"},
	}
	handler := &AssistantHandler{organizer: &fakeKnowledgeOrganizer{
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
	handler := &AssistantHandler{}
	if _, err := handler.saveInboxFallback(ctx, "user", "remember this"); !errors.Is(err, context.Canceled) {
		t.Fatalf("saveInboxFallback() error = %v, want context.Canceled", err)
	}
}

func TestQueryReceiptsCombinedQueryWithoutFiltersReturnsNoContext(t *testing.T) {
	handler := &AssistantHandler{}
	receipts, err := handler.queryReceipts("user", &models.AssistantIntentResponse{Type: "combined_query"})
	if err != nil {
		t.Fatalf("queryReceipts() error = %v", err)
	}
	if receipts != nil {
		t.Fatalf("queryReceipts() = %#v, want nil when no receipt query was planned", receipts)
	}

	receipts, err = handler.queryReceipts("user", &models.AssistantIntentResponse{Type: "receipt_query"})
	if err == nil {
		t.Fatalf("receipt-only queryReceipts() = %#v, want missing-filter error", receipts)
	}
}

func TestFormatKnowledgeResultsDistinguishesReceiptContext(t *testing.T) {
	result := models.KnowledgeSearchResult{
		Entry: models.KnowledgeEntry{Title: "Milk preference", Body: "Prefers whole milk."},
	}
	emptyReceipts := &models.CompactReceiptResponse{Receipts: []models.CompactReceipt{}}
	matchingReceipts := &models.CompactReceiptResponse{Receipts: []models.CompactReceipt{{ID: "receipt-1"}}}

	if got := formatKnowledgeResults(nil, nil); strings.Contains(strings.ToLower(got), "receipt") {
		t.Fatalf("not-queried fallback unexpectedly mentions receipts: %q", got)
	}
	if got := formatKnowledgeResults(nil, emptyReceipts); got != "I found no matching personal knowledge or receipts." {
		t.Fatalf("zero-match fallback = %q", got)
	}
	if got := formatKnowledgeResults([]models.KnowledgeSearchResult{result}, emptyReceipts); !strings.Contains(got, "No matching receipts were found.") {
		t.Fatalf("zero-match fallback does not report receipt query result: %q", got)
	}
	if got := formatKnowledgeResults([]models.KnowledgeSearchResult{result}, matchingReceipts); !strings.Contains(got, "combined receipt summary") {
		t.Fatalf("matching-receipt fallback does not report degraded summary: %q", got)
	}
}

func TestEnrichReceiptFiltersFromKnowledgeAddsStrictProductAndBrandAttributes(t *testing.T) {
	intent := &models.AssistantIntentResponse{
		Type: "combined_query",
		Specific: &models.AssistantQueryFilter{
			ProductName:  []string{"milk"},
			ProductScope: "generic",
		},
		General: &models.AssistantQueryFilter{
			ProductName:  []string{"milk"},
			ProductScope: "generic",
		},
	}
	results := []models.KnowledgeSearchResult{{
		Entry: models.KnowledgeEntry{
			Attributes: models.KnowledgeAttributes{
				"product": "Milk",
				"brand":   "Brand X",
			},
		},
	}}

	enriched := EnrichReceiptFiltersFromKnowledge(intent, results)
	if got := enriched.Specific.Brand; len(got) != 1 || got[0] != "Brand X" {
		t.Fatalf("specific brands = %#v, want Brand X", got)
	}
	if got := enriched.General.Brand; len(got) != 1 || got[0] != "Brand X" {
		t.Fatalf("general brands = %#v, want Brand X", got)
	}
	if len(enriched.Specific.ProductName) != 1 || enriched.Specific.ProductName[0] != "milk" {
		t.Fatalf("existing exact product filter was broadened: %#v", enriched.Specific.ProductName)
	}
	if len(intent.Specific.Brand) != 0 {
		t.Fatal("enrichment mutated the classifier intent")
	}
}

func TestEnrichReceiptFiltersFromKnowledgeCreatesQueryOnlyWhenAttributesExist(t *testing.T) {
	intent := &models.AssistantIntentResponse{
		Type:      "combined_query",
		Knowledge: &models.AssistantKnowledgeAction{Operation: "search"},
	}
	unchanged := EnrichReceiptFiltersFromKnowledge(intent, nil)
	if unchanged.Specific != nil || unchanged.General != nil {
		t.Fatalf("filters without product attributes = %#v / %#v, want nil", unchanged.Specific, unchanged.General)
	}

	results := []models.KnowledgeSearchResult{{
		Entry: models.KnowledgeEntry{Attributes: models.KnowledgeAttributes{
			"product_name": "Whole milk",
			"brand":        "Brand X",
		}},
	}}
	enriched := EnrichReceiptFiltersFromKnowledge(intent, results)
	if enriched.Specific == nil || len(enriched.Specific.ProductName) != 1 || len(enriched.Specific.Brand) != 1 {
		t.Fatalf("attribute-driven filters = %#v", enriched.Specific)
	}
}
