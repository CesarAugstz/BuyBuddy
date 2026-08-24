package utils

import (
	"buybuddy-api/models"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestBuildIntentPromptRejectsRelatedProductsAndClassifiesScope(t *testing.T) {
	prompt := buildIntentPrompt(
		"Quanto paguei em cerveja?",
		nil,
		nil,
		nil,
		time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC),
	)

	for _, expected := range []string{
		`"productScope": "specific" | "generic"`,
		"Never add merely related",
		`"cerveja" may include beer brands but not wine, soda, or snacks`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt does not contain %q", expected)
		}
	}
}

func TestBuildIntentPromptConstrainsKnowledgeOperations(t *testing.T) {
	context := &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Projects"}},
		Entries: []models.KnowledgeAssistantEntry{{
			ID:      "entry-1",
			TopicID: "topic-1",
			Title:   "BuyBuddy decision",
			Version: 2,
		}},
	}
	prompt := buildIntentPrompt(
		"Change my BuyBuddy decision",
		nil,
		nil,
		nil,
		time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
		context,
	)
	for _, expected := range []string{
		`"type": "knowledge_query" | "knowledge_change" | "knowledge_forget"`,
		`"type": "knowledge_organize"`,
		`"operation": "organize"`,
		"exactly one supplied topic path clearly matches",
		"Clean up my Inbox",
		"never return SQL",
		"Never choose multiple entries",
		`"entry-1"`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt does not contain %q", expected)
		}
	}
}

func TestParseIntentResponseAcceptsStructuredKnowledgeAndRejectsUnknownType(t *testing.T) {
	intent, err := parseIntentResponse(`{
		"type":"knowledge_change",
		"confidence":"high",
		"knowledge":{"operation":"update","entryId":"entry-1","expectedVersion":2,"title":"New title"}
	}`)
	if err != nil {
		t.Fatalf("parseIntentResponse() error = %v", err)
	}
	if intent.Knowledge == nil || intent.Knowledge.EntryID != "entry-1" || intent.Knowledge.ExpectedVersion != 2 {
		t.Fatalf("parsed intent = %#v", intent)
	}
	organize, err := parseIntentResponse(`{
		"type":"knowledge_organize",
		"confidence":"high",
		"knowledge":{"operation":"organize","topicId":"topic-1"}
	}`)
	if err != nil || organize.Knowledge == nil || organize.Knowledge.Operation != "organize" {
		t.Fatalf("parsed organize intent/error = %#v/%v", organize, err)
	}
	if _, err := parseIntentResponse(`{"type":"run_sql"}`); err == nil {
		t.Fatal("unknown intent type unexpectedly accepted")
	}
}

func TestParseIntentResponseRejectsReceiptQueryWithoutFilters(t *testing.T) {
	if _, err := parseIntentResponse(`{"type":"receipt_query","confidence":"high"}`); err == nil {
		t.Fatal("filterless receipt_query unexpectedly accepted")
	}
	if _, err := parseIntentResponse(`{"type":"receipt_query","confidence":"high","specific":{}}`); err == nil {
		t.Fatal("empty receipt_query filter unexpectedly accepted")
	}

	limit := 1
	intent, err := parseIntentResponse(fmt.Sprintf(
		`{"type":"receipt_query","confidence":"high","specific":{"limit":%d,"orderBy":"date_desc","returnFullReceipt":true}}`,
		limit,
	))
	if err != nil {
		t.Fatalf("valid receipt_query rejected: %v", err)
	}
	if intent.Specific == nil || intent.Specific.Limit == nil || *intent.Specific.Limit != 1 {
		t.Fatalf("parsed intent = %#v", intent)
	}
}

func TestBuildIntentCorrectionPromptIncludesSemanticFailure(t *testing.T) {
	prompt := buildIntentCorrectionPrompt(
		"original instructions",
		`{"type":"receipt_query"}`,
		fmt.Errorf("receipt_query requires a filter"),
	)
	for _, expected := range []string{
		"original instructions",
		`{"type":"receipt_query"}`,
		"receipt_query requires a filter",
		"complete corrected JSON object",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("correction prompt does not contain %q", expected)
		}
	}
}

func TestReceiptIntentCorrectionSchemaRequiresQueryPlan(t *testing.T) {
	schema := assistantReceiptIntentCorrectionJSONSchema()
	required, ok := schema["required"].([]string)
	if !ok || !slices.Contains(required, "specific") {
		t.Fatalf("top-level required fields = %#v, want specific", schema["required"])
	}
	properties := schema["properties"].(map[string]interface{})
	specific := properties["specific"].(map[string]interface{})
	filterRequired, ok := specific["required"].([]string)
	if !ok {
		t.Fatalf("specific required fields = %#v", specific["required"])
	}
	for _, field := range []string{"limit", "orderBy", "returnFullReceipt"} {
		if !slices.Contains(filterRequired, field) {
			t.Errorf("specific schema does not require %q: %#v", field, filterRequired)
		}
	}
}

func TestAssistantIntentSchemaDoesNotExposeSQLOrUserID(t *testing.T) {
	schema := assistantIntentJSONSchema()
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	lower := strings.ToLower(string(encoded))
	if strings.Contains(lower, `"sql"`) || strings.Contains(lower, `"userid"`) {
		t.Fatalf("assistant intent schema exposes forbidden fields: %s", encoded)
	}
	if !strings.Contains(lower, `"knowledge_organize"`) || !strings.Contains(lower, `"organize"`) {
		t.Fatalf("assistant intent schema does not expose bounded organize action: %s", encoded)
	}
}

func TestBuildConversationContextBoundsMessagesAndContent(t *testing.T) {
	messages := make([]models.ChatMessage, 25)
	for i := range messages {
		messages[i] = models.ChatMessage{Role: "user", Content: strings.Repeat("x", 3000)}
	}
	context := buildConversationContext(messages)
	if strings.Count(context, "User:") != 6 {
		t.Fatalf("message count in bounded context = %d, want 6 within 12000 content runes", strings.Count(context, "User:"))
	}
	if len([]rune(context)) > 12100 {
		t.Fatalf("bounded conversation has %d runes, want near 12000 maximum", len([]rune(context)))
	}
}

func TestNormalizeQuestionSuggestionsRequiresPlaceholdersAndDeduplicates(t *testing.T) {
	defaults := DefaultQuestionSuggestions(nil)
	suggestions := normalizeQuestionSuggestions([]string{
		"How much did I pay for {item}?",
		"How much did I pay for {item}?",
		"This suggestion has no placeholder",
		"Where did I buy {item}?",
	}, defaults)

	if len(suggestions) != 5 {
		t.Fatalf("len(suggestions) = %d, want 5", len(suggestions))
	}
	if suggestions[0] != "How much did I pay for {item}?" {
		t.Errorf("first suggestion = %q", suggestions[0])
	}
	for _, suggestion := range suggestions {
		if !strings.Contains(suggestion, "{") {
			t.Errorf("suggestion has no placeholder: %q", suggestion)
		}
	}
}

func TestDefaultQuestionSuggestionsUsesRecentQuestionLanguage(t *testing.T) {
	suggestions := DefaultQuestionSuggestions([]string{
		"Quanto paguei no leite?",
		"Onde comprei café?",
	})

	if len(suggestions) == 0 || !strings.Contains(suggestions[0], "Quanto") {
		t.Fatalf("unexpected Portuguese suggestions: %#v", suggestions)
	}
}

func TestBoundRecentQuestionsLimitsCountAndSize(t *testing.T) {
	questions := make([]string, 25)
	for index := range questions {
		questions[index] = strings.Repeat("a", 700)
	}

	bounded := boundRecentQuestions(questions)

	if len(bounded) != 12 {
		t.Fatalf("len(bounded) = %d, want 12 within the 6000-rune total", len(bounded))
	}
	total := 0
	for _, question := range bounded {
		runeCount := len([]rune(question))
		if runeCount > 500 {
			t.Errorf("question has %d runes, want at most 500", runeCount)
		}
		total += runeCount
	}
	if total != 6000 {
		t.Errorf("total runes = %d, want 6000", total)
	}
}

func TestFormatReceiptsCompactGroupsGenericProductMatches(t *testing.T) {
	recent := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	older := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	receipts := []models.Receipt{
		{
			ID:      "receipt-1",
			Company: "Mercado A",
			Date:    &recent,
			Items: []models.ReceiptItem{
				{Name: "Cerveja Eisenbahn", RawName: "CERV.EISENBAHN", Brand: "Eisenbahn", Quantity: 2, Unit: "un", UnitPrice: 4, TotalPrice: 8},
				{Name: "Batata Chips", RawName: "BATATA", Brand: "Marca X", Quantity: 1, Unit: "un", UnitPrice: 10, TotalPrice: 10},
			},
		},
		{
			ID:      "receipt-2",
			Company: "Mercado B",
			Date:    &older,
			Items: []models.ReceiptItem{
				{Name: "Cerveja Eisenbahn", RawName: "EISENBAHN LONG", Brand: "Eisenbahn", Quantity: 1, Unit: "un", UnitPrice: 5, TotalPrice: 5},
				{Name: "Cerveja Spaten", RawName: "CERV.SPATEN", Brand: "Spaten", Quantity: 1, Unit: "un", UnitPrice: 3.5, TotalPrice: 3.5},
			},
		},
	}
	filter := &models.AssistantQueryFilter{
		ProductName:    []string{"cerveja"},
		ProductScope:   "generic",
		ProductConcept: "cerveja",
	}

	response := FormatReceiptsCompact(receipts, filter)

	if response.ProductScope != "generic" {
		t.Errorf("ProductScope = %q", response.ProductScope)
	}
	if len(response.ProductGroups) != 2 {
		t.Fatalf("len(ProductGroups) = %d, want 2", len(response.ProductGroups))
	}

	eisenbahn := response.ProductGroups[0]
	if eisenbahn.Name != "Cerveja Eisenbahn" ||
		eisenbahn.PurchaseCount != 2 ||
		eisenbahn.TotalQuantity != 3 ||
		eisenbahn.AverageUnitPrice != 13.0/3.0 ||
		eisenbahn.MinimumUnitPrice != 4 ||
		eisenbahn.MaximumUnitPrice != 5 ||
		eisenbahn.LatestUnitPrice != 4 ||
		eisenbahn.LatestDate != "2026-08-02" {
		t.Errorf("unexpected Eisenbahn group: %+v", eisenbahn)
	}

	for _, receipt := range response.Receipts {
		for _, item := range receipt.Items {
			if item.Name == "Batata Chips" {
				t.Error("unrelated product was included in compact results")
			}
		}
	}
}

func TestFormatReceiptsCompactDoesNotGroupSpecificProductQuery(t *testing.T) {
	filter := &models.AssistantQueryFilter{
		ProductName:    []string{"Eisenbahn"},
		ProductScope:   "specific",
		ProductConcept: "Cerveja Eisenbahn",
	}

	response := FormatReceiptsCompact(nil, filter)
	if len(response.ProductGroups) != 0 {
		t.Errorf("specific query produced %d product groups", len(response.ProductGroups))
	}
}

func TestItemMatchesFilterRequiresRequestedBrand(t *testing.T) {
	filter := &models.AssistantQueryFilter{
		ProductName: []string{"cerveja"},
		Brand:       []string{"Eisenbahn"},
	}

	if !itemMatchesFilter(models.ReceiptItem{Name: "Cerveja", Brand: "Eisenbahn"}, filter) {
		t.Error("requested brand did not match")
	}
	if itemMatchesFilter(models.ReceiptItem{Name: "Cerveja", Brand: "Spaten"}, filter) {
		t.Error("related product with a different brand matched")
	}
}
