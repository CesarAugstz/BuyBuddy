package utils

import (
	"buybuddy-api/models"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"
)

func TestFirstCandidateTextSafelyHandlesMissingContent(t *testing.T) {
	tests := []*genai.GenerateContentResponse{
		nil,
		{},
		{Candidates: []*genai.Candidate{nil}},
		{Candidates: []*genai.Candidate{{Content: nil}}},
		{Candidates: []*genai.Candidate{{Content: &genai.Content{}}}},
		{Candidates: []*genai.Candidate{{Content: &genai.Content{Parts: []*genai.Part{nil}}}}},
	}
	for index, response := range tests {
		if text, ok := firstCandidateText(response); ok || text != "" {
			t.Errorf("case %d firstCandidateText() = %q, %t; want empty, false", index, text, ok)
		}
	}

	response := &genai.GenerateContentResponse{Candidates: []*genai.Candidate{{
		Content: &genai.Content{Parts: []*genai.Part{nil, {Text: `{"type":"direct"}`}}},
	}}}
	if text, ok := firstCandidateText(response); !ok || text != `{"type":"direct"}` {
		t.Fatalf("firstCandidateText() = %q, %t", text, ok)
	}
}

func TestBuildIntentPromptRejectsRelatedProductsAndClassifiesScope(t *testing.T) {
	prompt := buildReceiptIntentPrompt(
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

func TestReceiptIntentPromptExcludesKnowledgeOperations(t *testing.T) {
	context := &models.KnowledgeAssistantContext{
		Topics: []models.KnowledgeAssistantTopic{{ID: "topic-1", Path: "Projects"}},
		Entries: []models.KnowledgeAssistantEntry{{
			ID:      "entry-1",
			TopicID: "topic-1",
			Title:   "BuyBuddy decision",
			Version: 2,
		}},
	}
	_ = context
	prompt := buildReceiptIntentPrompt(
		"How much did I pay for milk last time?",
		nil,
		nil,
		nil,
		time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
	)
	for _, expected := range []string{
		`"type": "receipt_query"`,
		`"last purchase" → limit: 1, orderBy: "date_desc"`,
		"Never create, update, delete, search, or organize personal knowledge",
		"never return SQL",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt does not contain %q", expected)
		}
	}
	for _, forbidden := range []string{"knowledge_write", "knowledge_query", "combined_query", `"entry-1"`} {
		if strings.Contains(prompt, forbidden) {
			t.Errorf("receipt prompt contains forbidden knowledge contract %q", forbidden)
		}
	}
}

func TestParseKnowledgeIntentResponseAcceptsStructuredKnowledgeAndRejectsReceiptType(t *testing.T) {
	intent, err := parseKnowledgeIntentResponse(`{
		"type":"knowledge_change",
		"confidence":"high",
		"knowledge":{"operation":"update","entryId":"entry-1","expectedVersion":2,"title":"New title"}
	}`)
	if err != nil {
		t.Fatalf("parseKnowledgeIntentResponse() error = %v", err)
	}
	if intent.Knowledge == nil || intent.Knowledge.EntryID != "entry-1" || intent.Knowledge.ExpectedVersion != 2 {
		t.Fatalf("parsed intent = %#v", intent)
	}
	organize, err := parseKnowledgeIntentResponse(`{
		"type":"knowledge_organize",
		"confidence":"high",
		"knowledge":{"operation":"organize","topicId":"topic-1"}
	}`)
	if err != nil || organize.Knowledge == nil || organize.Knowledge.Operation != "organize" {
		t.Fatalf("parsed organize intent/error = %#v/%v", organize, err)
	}
	if _, err := parseKnowledgeIntentResponse(`{"type":"receipt_query","knowledge":{"operation":"search"}}`); err == nil {
		t.Fatal("receipt intent unexpectedly accepted by knowledge parser")
	}
	if _, err := parseKnowledgeIntentResponse(`{"type":"run_sql"}`); err == nil {
		t.Fatal("unknown intent type unexpectedly accepted")
	}
}

func TestInspectKnowledgeIntentResponseLogsMetadataWithoutValues(t *testing.T) {
	response := `{
		"type":"knowledge_query",
		"confidence":"high",
		"knowledge":{
			"operation":"create",
			"entryId":"private-entry-id",
			"topicId":"private-topic-id",
			"searchQuery":"private search terms"
		}
	}`

	diagnostic := inspectKnowledgeIntentResponse(response)

	if diagnostic.Type != "knowledge_query" ||
		diagnostic.Operation != "create" ||
		diagnostic.Confidence != "high" ||
		!diagnostic.HasKnowledge ||
		!diagnostic.EntryIDPresent ||
		!diagnostic.TopicIDPresent ||
		diagnostic.SearchQueryChars != len([]rune("private search terms")) ||
		len(diagnostic.Fingerprint) != 12 {
		t.Fatalf("unexpected diagnostic: %#v", diagnostic)
	}
	rendered := fmt.Sprintf("%+v", diagnostic)
	for _, privateValue := range []string{
		"private-entry-id",
		"private-topic-id",
		"private search terms",
	} {
		if strings.Contains(rendered, privateValue) {
			t.Errorf("diagnostic leaked %q: %s", privateValue, rendered)
		}
	}
}

func TestKnowledgeIntentCorrectionSchemaRequiresMatchingOperation(t *testing.T) {
	schema := knowledgeAssistantIntentCorrectionJSONSchema(
		"knowledge_query",
		"search",
	)
	required := schema["required"].([]string)
	if !slices.Contains(required, "knowledge") {
		t.Fatalf("top-level required fields = %#v, want knowledge", required)
	}
	properties := schema["properties"].(map[string]interface{})
	intentType := properties["type"].(map[string]interface{})
	if got := intentType["enum"].([]string); len(got) != 1 || got[0] != "knowledge_query" {
		t.Fatalf("intent type enum = %#v", got)
	}
	knowledge := properties["knowledge"].(map[string]interface{})
	knowledgeProperties := knowledge["properties"].(map[string]interface{})
	operation := knowledgeProperties["operation"].(map[string]interface{})
	if got := operation["enum"].([]string); len(got) != 1 || got[0] != "search" {
		t.Fatalf("operation enum = %#v", got)
	}
}

func TestParseReceiptIntentResponseRejectsKnowledgeAndQueryWithoutFilters(t *testing.T) {
	if _, err := parseReceiptIntentResponse(`{"type":"knowledge_write"}`); err == nil {
		t.Fatal("knowledge intent unexpectedly accepted by receipt parser")
	}
	if _, err := parseReceiptIntentResponse(`{"type":"receipt_query","confidence":"high"}`); err == nil {
		t.Fatal("filterless receipt_query unexpectedly accepted")
	}
	if _, err := parseReceiptIntentResponse(`{"type":"receipt_query","confidence":"high","specific":{}}`); err == nil {
		t.Fatal("empty receipt_query filter unexpectedly accepted")
	}

	limit := 1
	intent, err := parseReceiptIntentResponse(fmt.Sprintf(
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

func TestLatestPurchaseSemanticCorrectionProducesExecutableReceiptQuery(t *testing.T) {
	_, semanticErr := parseReceiptIntentResponse(`{"type":"receipt_query"}`)
	if !errors.Is(semanticErr, errReceiptQueryMissingFilter) {
		t.Fatalf("filterless latest-purchase intent error = %v", semanticErr)
	}
	original := buildReceiptIntentPrompt(
		"When did I last buy milk?",
		nil,
		nil,
		nil,
		time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
	)
	correction := buildIntentCorrectionPrompt(
		original,
		`{"type":"receipt_query"}`,
		semanticErr,
	)
	if !strings.Contains(correction, "When did I last buy milk?") {
		t.Fatal("semantic correction lost the original latest-purchase question")
	}

	intent, err := parseReceiptIntentResponse(`{
		"type":"receipt_query",
		"specific":{
			"productName":["milk"],
			"limit":1,
			"orderBy":"date_desc",
			"returnFullReceipt":false
		}
	}`)
	if err != nil {
		t.Fatalf("corrected latest-purchase intent rejected: %v", err)
	}
	if intent.Specific == nil || intent.Specific.Limit == nil ||
		*intent.Specific.Limit != 1 || intent.Specific.OrderBy != "date_desc" {
		t.Fatalf("corrected latest-purchase intent = %#v", intent)
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

func TestReceiptAssistantIntentSchemaExcludesKnowledgeSQLAndUserID(t *testing.T) {
	schema := receiptAssistantIntentJSONSchema()
	encoded, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	lower := strings.ToLower(string(encoded))
	if strings.Contains(lower, `"sql"`) || strings.Contains(lower, `"userid"`) ||
		strings.Contains(lower, `"knowledge"`) || strings.Contains(lower, `"combined_query"`) {
		t.Fatalf("assistant intent schema exposes forbidden fields: %s", encoded)
	}
	if !strings.Contains(lower, `"receipt_query"`) || !strings.Contains(lower, `"direct"`) {
		t.Fatalf("receipt assistant schema is missing receipt intents: %s", encoded)
	}
}

func TestKnowledgeAssistantIntentSchemaExcludesReceiptAndCombinedTypes(t *testing.T) {
	encoded, err := json.Marshal(knowledgeAssistantIntentJSONSchema())
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"receipt_query", "combined_query", "specific", "general", `"sql"`} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("knowledge assistant schema contains %q: %s", forbidden, encoded)
		}
	}
	for _, expected := range []string{"knowledge_write", "knowledge_query", "knowledge_change", "knowledge_forget", "knowledge_organize"} {
		if !strings.Contains(lower, expected) {
			t.Errorf("knowledge assistant schema is missing %q", expected)
		}
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
