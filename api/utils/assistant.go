package utils

import (
	"buybuddy-api/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"
)

const cheapModel = "gemini-2.5-flash-lite"

const schemaDescription = `Database schema for user receipts:

RECEIPTS table:
- id: unique identifier
- company: store/company name where purchase was made
- date: purchase date (YYYY-MM-DD format)
- total: total amount paid

RECEIPT_ITEMS table (each receipt has multiple items):
- name: cleaned product name
- raw_name: original product name from receipt
- brand: product brand (may be empty)
- quantity: amount purchased
- unit: unit of measurement (un, kg, L, etc.)
- unit_price: price per unit
- total_price: total price for this item
- category: product category name
- subcategory: product subcategory name
- barcode: product barcode (may be empty)`

func createGeminiClient(ctx context.Context, apiKey string) (*genai.Client, error) {
	return genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
}

func buildConversationContext(conversationHistory []models.ChatMessage) string {
	if len(conversationHistory) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\nPrevious conversation:\n")
	for _, msg := range conversationHistory {
		if msg.Role == "user" {
			sb.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
		} else {
			sb.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content))
		}
	}
	return sb.String()
}

func buildCategoryList(categories []models.Category) string {
	if len(categories) == 0 {
		return "No categories available."
	}

	var sb strings.Builder
	sb.WriteString("Available categories and subcategories:\n")
	for _, cat := range categories {
		sb.WriteString(fmt.Sprintf("- %s", cat.Name))
		if len(cat.Subcategories) > 0 {
			subNames := make([]string, 0, len(cat.Subcategories))
			for _, sub := range cat.Subcategories {
				subNames = append(subNames, sub.Name)
			}
			sb.WriteString(fmt.Sprintf(": %s", strings.Join(subNames, ", ")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func buildIntentPrompt(question string, conversationHistory []models.ChatMessage, firstReceiptDate *time.Time, categories []models.Category, currentTime time.Time) string {
	conversationContext := buildConversationContext(conversationHistory)

	firstReceiptInfo := "No receipts yet."
	if firstReceiptDate != nil {
		firstReceiptInfo = fmt.Sprintf("User's first receipt date: %s", firstReceiptDate.Format("2006-01-02"))
	}

	categoryList := buildCategoryList(categories)

	return fmt.Sprintf(`You are a shopping assistant that helps users query their purchase history.

%s

%s

Current context:
- Current date: %s
- Day of week: %s
- Timezone: Brasília (GMT-3)
- %s
%s

User's question: %s

Analyze if this question requires querying the receipt database or can be answered directly.

RESPOND WITH JSON ONLY. Choose one of these formats:

OPTION A - Direct answer (for greetings, general questions, or questions answerable from conversation history):
{
  "type": "direct",
  "answer": "Your helpful response here"
}

OPTION B - Query needed (for questions about purchases, prices, products, spending):
{
  "type": "query",
  "specific": {
    "productName": ["exact product name or 1-2 close variations"],
    "company": ["store name if mentioned"],
    "brand": ["brand if mentioned"],
    "category": ["category if relevant"],
    "subcategory": ["subcategory if relevant"],
    "dateFrom": "YYYY-MM-DD if date range mentioned",
    "dateTo": "YYYY-MM-DD if date range mentioned",
    "minPrice": null or number,
    "maxPrice": null or number,
    "limit": number (how many results needed, e.g., 1 for "last purchase", 5 for "last 5", null for all),
    "orderBy": "date_desc" | "date_asc" | "total_desc" | "total_asc" (default: date_desc),
    "returnFullReceipt": false (set to true ONLY if user needs to see ALL items from matching receipts, not just the queried products)
  },
  "general": {
    "productName": ["broader variations, synonyms, related terms - 3-5 options"],
    "company": ["if mentioned, keep same"],
    "brand": ["if mentioned, keep same or add variations"],
    "category": ["broader category if relevant"],
    "subcategory": [],
    "dateFrom": "wider date range or null",
    "dateTo": "wider date range or null",
    "minPrice": null,
    "maxPrice": null,
    "limit": null or larger number than specific,
    "orderBy": same as specific or null
  }
}

IMPORTANT NOTES:
- When searching for multiple specific product names (e.g., "patinho bovino", "leite"), the category filter will be ignored automatically since products span multiple categories
- Use returnFullReceipt: true only when user asks something like "what else did I buy with X" or "show me the full receipt"

LIMIT AND ORDER EXAMPLES:
- "last purchase" → limit: 1, orderBy: "date_desc"
- "first time I bought" → limit: 1, orderBy: "date_asc"
- "last 3 times" → limit: 3, orderBy: "date_desc"
- "most expensive purchase" → limit: 1, orderBy: "total_desc"
- "cheapest milk" → limit: 1, orderBy: "total_asc"
- Recipe/cost estimation with multiple ingredients → limit: 10-20 (need price history for each ingredient)
- Price comparison questions → limit: 5-10 (need multiple purchases to compare)

For the general query, make it less restrictive than specific:
- Add more product name variations and synonyms
- Widen or remove date constraints
- Remove price constraints
- Keep only essential filters

Only include non-empty fields. Omit fields with empty arrays or null values.`, schemaDescription, categoryList, currentTime.Format("2006-01-02"), currentTime.Weekday().String(), firstReceiptInfo, conversationContext, question)
}

func parseIntentResponse(response string) (*models.AssistantIntentResponse, error) {
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var intent models.AssistantIntentResponse
	if err := json.Unmarshal([]byte(response), &intent); err != nil {
		return nil, fmt.Errorf("failed to parse intent response: %w", err)
	}
	return &intent, nil
}

func DetectIntentAndGenerateQuery(ctx context.Context, question string, conversationHistory []models.ChatMessage, firstReceiptDate *time.Time, categories []models.Category, apiKey string) (*models.AssistantIntentResponse, error) {
	client, err := createGeminiClient(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	brasilia := time.FixedZone("BRT", -3*60*60)
	currentTime := time.Now().In(brasilia)

	prompt := buildIntentPrompt(question, conversationHistory, firstReceiptDate, categories, currentTime)

	log.Println("Intent detection prompt:", prompt)

	var intent *models.AssistantIntentResponse
	var lastErr error

	for attempt := 0; attempt < 2; attempt++ {
		resp, err := client.Models.GenerateContent(ctx, cheapModel, []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: prompt},
				},
			},
		}, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to generate content: %w", err)
			continue
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			lastErr = fmt.Errorf("empty response from model")
			continue
		}

		text := resp.Candidates[0].Content.Parts[0].Text
		log.Println("Intent detection response:", text)
		intent, err = parseIntentResponse(text)
		if err != nil {
			lastErr = err
			continue
		}

		return intent, nil
	}

	return nil, fmt.Errorf("failed after 2 attempts: %w", lastErr)
}

func FormatReceiptsCompact(receipts []models.Receipt, filter *models.AssistantQueryFilter) *models.CompactReceiptResponse {
	legend := map[string]string{
		"co":  "company",
		"d":   "date",
		"t":   "total",
		"n":   "name",
		"rn":  "rawName",
		"b":   "brand",
		"q":   "quantity",
		"u":   "unit",
		"up":  "unitPrice",
		"tp":  "totalPrice",
		"cat": "category",
		"sc":  "subcategory",
		"bc":  "barcode",
	}

	shouldFilterItems := filter != nil && !filter.ReturnFullReceipt && len(filter.ProductName) > 0

	compactReceipts := make([]models.CompactReceipt, 0, len(receipts))
	for _, r := range receipts {
		cr := models.CompactReceipt{
			ID:      r.ID,
			Company: strings.TrimSpace(r.Company),
			Total:   r.Total,
		}
		if r.Date != nil {
			cr.Date = r.Date.Format("2006-01-02")
		}

		items := make([]models.CompactReceiptItem, 0, len(r.Items))
		for _, item := range r.Items {
			if shouldFilterItems && !itemMatchesFilter(item, filter) {
				continue
			}

			ci := models.CompactReceiptItem{
				Name:    strings.TrimSpace(item.Name),
				RawName: strings.TrimSpace(item.RawName),
				Qty:     item.Quantity,
				Unit:    strings.TrimSpace(item.Unit),
				UP:      item.UnitPrice,
				TP:      item.TotalPrice,
			}
			if brand := strings.TrimSpace(item.Brand); brand != "" {
				ci.Brand = brand
			}
			if item.Category != nil {
				ci.Cat = strings.TrimSpace(item.Category.Name)
			}
			if item.Subcategory != nil {
				ci.SubCat = strings.TrimSpace(item.Subcategory.Name)
			}
			if barcode := strings.TrimSpace(item.Barcode); barcode != "" {
				ci.Barcode = barcode
			}
			items = append(items, ci)
		}
		if len(items) > 0 || !shouldFilterItems {
			cr.Items = items
			compactReceipts = append(compactReceipts, cr)
		}
	}

	return &models.CompactReceiptResponse{
		Legend:   legend,
		Receipts: compactReceipts,
	}
}

func itemMatchesFilter(item models.ReceiptItem, filter *models.AssistantQueryFilter) bool {
	for _, name := range filter.ProductName {
		nameLower := strings.ToLower(name)
		if strings.Contains(strings.ToLower(item.Name), nameLower) ||
			strings.Contains(strings.ToLower(item.RawName), nameLower) {
			return true
		}
	}
	return false
}

func MergeResults(specific, general []models.Receipt) []models.Receipt {
	if len(specific) >= 10 {
		return specific
	}

	seenIDs := make(map[string]bool)
	for _, r := range specific {
		seenIDs[r.ID] = true
	}

	result := make([]models.Receipt, len(specific))
	copy(result, specific)

	added := 0
	for _, r := range general {
		if added >= 5 {
			break
		}
		if !seenIDs[r.ID] {
			result = append(result, r)
			seenIDs[r.ID] = true
			added++
		}
	}

	return result
}

func GenerateAnswer(ctx context.Context, question string, receipts *models.CompactReceiptResponse, conversationHistory []models.ChatMessage, apiKey string, modelName string) (string, error) {
	client, err := createGeminiClient(ctx, apiKey)
	if err != nil {
		return "", err
	}

	if modelName == "" {
		modelName = "gemini-2.5-flash-lite"
	}

	receiptsJSON, err := json.Marshal(receipts)
	if err != nil {
		return "", fmt.Errorf("failed to marshal receipts: %w", err)
	}

	conversationContext := buildConversationContext(conversationHistory)

	prompt := fmt.Sprintf(`You are a helpful shopping assistant for a Brazilian user.

The JSON below contains the user's relevant purchase history. Note the "_legend" field explains the abbreviations:
%s
%s

User's question: %s

IMPORTANT GUIDELINES:
- Show prices in Brazilian Reais (R$) with exact values
- Include store name and date when discussing purchases
- If no relevant data found, tell the user you don't have that information
- Use conversation context for references like "that product" or "the last one"
- When counting "how many times" user bought something, count RECEIPTS (separate purchases/dates), not line items
- Each receipt ID represents one purchase occasion, even if the same product appears multiple times in one receipt

WHEN PROVIDING PRODUCT HISTORY:
- Product name and brand (if available)
- Store/Company name and purchase date
- Quantity and unit (kg, un, L, etc.)
- Unit price and total price
- Category/subcategory if available
- Use markdown formatting (bold, lists)
- Show price comparisons for repeat purchases
- Highlight most recent purchase

Respond in the same language as the user's question. Be concise but informative.`, string(receiptsJSON), conversationContext, question)

	log.Println("Answer generation prompt:", prompt)
	resp, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: prompt},
			},
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "I'm sorry, I couldn't find an answer to your question.", nil
	}

	text := resp.Candidates[0].Content.Parts[0].Text
	if text == "" {
		return "I'm sorry, I couldn't process your request.", nil
	}

	return text, nil
}
