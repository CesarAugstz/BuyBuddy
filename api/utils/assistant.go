package utils

import (
	"buybuddy-api/models"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

func AskShoppingAssistant(ctx context.Context, question string, receipts []models.Receipt, conversationHistory []models.ChatMessage, apiKey string) (string, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-1.5-flash-8b")

	receiptsJSON, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal receipts: %w", err)
	}

	var conversationContext strings.Builder
	if len(conversationHistory) > 0 {
		conversationContext.WriteString("\n\nPrevious conversation:\n")
		for _, msg := range conversationHistory {
			if msg.Role == "user" {
				conversationContext.WriteString(fmt.Sprintf("User: %s\n", msg.Content))
			} else {
				conversationContext.WriteString(fmt.Sprintf("Assistant: %s\n", msg.Content))
			}
		}
		conversationContext.WriteString("\n")
	}

	prompt := fmt.Sprintf(`You are a helpful shopping assistant for a Brazilian user. 
The user has the following purchase history (receipts):

%s
%s
User's current question: %s

Analyze the receipts and conversation history to provide a helpful, conversational answer.

IMPORTANT GUIDELINES:
- If the user asks about prices, show them in Brazilian Reais (R$) with the exact values
- If the user asks about where they bought something, tell them the store name and date
- If the user asks about items they haven't purchased, tell them you don't have that information
- If the user refers to previous messages (like "that product" or "the last one"), use the conversation context

WHEN PROVIDING PRODUCT HISTORY:
- Include ALL relevant details for each purchase:
  * Product name and brand (if available)
  * Store/Company name
  * Purchase date
  * Quantity and unit (kg, un, L, etc.)
  * Unit price (price per kg, per unit, etc.)
  * Total price paid
  * Category and subcategory (if available)
- Format the information clearly using markdown (bold, lists, etc.)
- Show price comparisons if the user bought the same item multiple times
- Highlight the most recent purchase

Provide the answer in a friendly, natural way. Keep it concise but informative.
Always respond in the same language as the user's question.`, string(receiptsJSON), conversationContext.String(), question)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "I'm sorry, I couldn't find an answer to your question.", nil
	}

	textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "I'm sorry, I couldn't process your request.", nil
	}

	return string(textPart), nil
}
