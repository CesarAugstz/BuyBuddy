package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type ReceiptData struct {
	Company   string                   `json:"company"`
	Total     float64                  `json:"total"`
	AccessKey string                   `json:"accessKey"`
	Items     []map[string]interface{} `json:"items"`
}

type CategoryInfo struct {
	Name          string
	Subcategories []string
}

func ProcessReceiptWithGemini(ctx context.Context, imageData []byte, apiKey string, categories []CategoryInfo) (*ReceiptData, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-3-flash-preview")

	categoriesText := buildCategoriesText(categories)

	prompt := fmt.Sprintf(`You are an AI specialized in reading Brazilian receipts (notas fiscais).
Analyze this receipt image and extract the following information:
1. Company/Store name
2. Total amount
3. Access Key (Chave de Acesso) - a 44-digit numeric code found on NFe receipts
4. List of items with detailed information

IMPORTANT RULES:
- Only extract information that you can clearly read from the receipt
- If you cannot read or identify any field, return null for that field
- Do NOT make up or imagine any information
- For items, only include items you can clearly see
- Prices should be in decimal format (e.g., 10.50)
- The Access Key is a 44-digit code, usually labeled as "Chave de Acesso" or shown as a barcode number

For each item, you MUST extract at minimum:
- name: Product name (REQUIRED)
- totalPrice: Total price for this item (REQUIRED)

Additionally, extract if visible:
- brand: Brand name (if visible, otherwise null)
- quantity: Numeric quantity
- unit: Unit of measure ("kg", "un", "L", "g", "ml", "cx" for box, etc.)
- unitPrice: Price per unit (if visible, calculate from total/quantity if needed)
- category: Main category - MUST be one of the following categories
- subcategory: Subcategory - MUST be one of the subcategories for the chosen category

AVAILABLE CATEGORIES AND SUBCATEGORIES:
%s

Return the data in this exact JSON format:
{
  "company": "Company Name or null",
  "total": 0.00 or null,
  "accessKey": "44-digit number or null",
  "items": [
    {
      "name": "Item Name",
      "brand": "Brand Name or null",
      "quantity": 1.0,
      "unit": "un",
      "unitPrice": 0.00,
      "totalPrice": 0.00,
      "category": "Food",
      "subcategory": "Meat"
    }
  ]
}

CRITICAL: If you cannot extract the required fields (name and totalPrice) for any items, return an error:
{
  "error": "Could not extract required item information (name and price) from the receipt"
}`, categoriesText)

	fmt.Println("Prompt: %s", prompt)

	parts := []genai.Part{
		genai.Text(prompt),
		genai.ImageData("jpeg", imageData),
	}

	resp, err := model.GenerateContent(ctx, parts...)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected response format from Gemini")
	}

	responseText := string(textPart)

	var result struct {
		Error     string                   `json:"error"`
		Company   *string                  `json:"company"`
		Total     *float64                 `json:"total"`
		AccessKey *string                  `json:"accessKey"`
		Items     []map[string]interface{} `json:"items"`
	}

	cleanJSON := extractJSON(responseText)
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse Gemini response: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("gemini error: %s", result.Error)
	}

	if result.Company == nil && result.Total == nil {
		return nil, fmt.Errorf("insufficient data extracted from receipt")
	}

	receiptData := &ReceiptData{
		Items: result.Items,
	}

	if result.Company != nil {
		receiptData.Company = *result.Company
	} else {
		receiptData.Company = "Unknown Company"
	}

	if result.Total != nil {
		receiptData.Total = *result.Total
	}

	if result.AccessKey != nil {
		receiptData.AccessKey = *result.AccessKey
	}

	if len(receiptData.Items) == 0 {
		receiptData.Items = []map[string]interface{}{}
	}

	return receiptData, nil
}

func extractJSON(text string) string {
	start := -1
	end := -1

	for i, char := range text {
		if char == '{' && start == -1 {
			start = i
		}
		if char == '}' {
			end = i + 1
		}
	}

	if start != -1 && end != -1 && end > start {
		return text[start:end]
	}

	return text
}

func buildCategoriesText(categories []CategoryInfo) string {
	var builder strings.Builder

	for _, cat := range categories {
		builder.WriteString(fmt.Sprintf("\n- %s:", cat.Name))
		if len(cat.Subcategories) > 0 {
			builder.WriteString("\n  Subcategories: ")
			builder.WriteString(strings.Join(cat.Subcategories, ", "))
		}
	}

	return builder.String()
}
