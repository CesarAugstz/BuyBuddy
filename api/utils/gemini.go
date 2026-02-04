package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"
)

type ReceiptData struct {
	Company   string                   `json:"company"`
	Date      string                   `json:"date"`
	Total     float64                  `json:"total"`
	AccessKey string                   `json:"accessKey"`
	Items     []map[string]interface{} `json:"items"`
}

type CategoryInfo struct {
	Name          string
	Subcategories []string
}

type ItemMapping struct {
	RawName string
	Name    string
}

func ProcessReceiptWithGemini(ctx context.Context, imageData []byte, apiKey string, categories []CategoryInfo, itemMappings []ItemMapping, modelName string) (*ReceiptData, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	if modelName == "" {
		modelName = "gemini-2.5-flash"
	}

	categoriesText := buildCategoriesText(categories)
	itemMappingsText := buildItemMappingsText(itemMappings)

	prompt := fmt.Sprintf(`Você é uma IA especializada em ler notas fiscais brasileiras.
Analise esta imagem de nota fiscal e extraia as seguintes informações:
1. Nome da empresa/loja
2. Data e hora da compra
3. Valor total
4. Chave de Acesso - um código numérico de 44 dígitos encontrado em NFe
5. Lista de itens com informações detalhadas

REGRAS IMPORTANTES:
- Extraia apenas informações que você consegue ler claramente da nota fiscal
- Se não conseguir ler ou identificar algum campo, retorne null para esse campo
- NÃO invente ou imagine nenhuma informação
- Para itens, inclua apenas os que você consegue ver claramente
- Preços devem estar em formato decimal (ex: 10.50)
- Data deve estar no formato ISO 8601: "YYYY-MM-DDTHH:MM:SS" (ex: "2024-03-15T14:30:00")
- A Chave de Acesso é um código de 44 dígitos, geralmente rotulado como "Chave de Acesso" ou mostrado como número de código de barras

Para cada item, você DEVE extrair no mínimo:
- rawName: O nome EXATO do produto como escrito na nota fiscal, incluindo abreviações (OBRIGATÓRIO)
- nameOptions: Um array de 1-3 versões MELHORADAS e legíveis do nome do produto - expanda abreviações, corrija erros, deixe claro. A primeira opção deve ser a mais provável, seguida de alternativas se aplicável (OBRIGATÓRIO)
- totalPrice: Preço total deste item (OBRIGATÓRIO)

Adicionalmente, extraia se visível:
- brand: Nome da marca (se visível, caso contrário null)
- quantity: Quantidade numérica
- unit: Unidade de medida ("kg", "un", "L", "g", "ml", "cx" para caixa, etc.)
- unitPrice: Preço por unidade (se visível, calcule a partir de total/quantidade se necessário)
- categoryOptions: Array de 1-2 possíveis categorias com suas subcategorias em PORTUGUÊS. A primeira deve ser a mais provável. Formato: [{"category": "Alimentos", "subcategory": "Laticínios"}]

EXEMPLOS DE MELHORIA DE NOME DE PRODUTO:
- rawName: "LT UHT ITAMBE" → nameOptions: ["Leite UHT Itambé"]
- rawName: "ARROZ TIPO 1" → nameOptions: ["Arroz Tipo 1", "Arroz Branco Tipo 1"]
- rawName: "FGO PRETO" → nameOptions: ["Feijão Preto"]
- rawName: "CAFE PILAO" → nameOptions: ["Café Pilão", "Café em Pó Pilão"]
- rawName: "REFRIGERANTE COCA" → nameOptions: ["Refrigerante Coca-Cola", "Coca-Cola"]
- rawName: "QJO MINAS" → nameOptions: ["Queijo Minas", "Queijo Minas Frescal"]

%s

CATEGORIAS E SUBCATEGORIAS DISPONÍVEIS (EM PORTUGUÊS):
%s

Retorne os dados neste formato JSON exato:
{
  "company": "Company Name or null",
  "date": "2024-03-15T14:30:00 or null",
  "total": 0.00 or null,
  "accessKey": "44-digit number or null",
  "items": [
    {
      "rawName": "LT UHT ITAMBE",
      "nameOptions": ["Leite UHT Itambé", "Leite Longa Vida Itambé"],
      "brand": "Itambé",
      "quantity": 1.0,
      "unit": "un",
      "unitPrice": 0.00,
      "totalPrice": 0.00,
      "categoryOptions": [
        {"category": "Laticínios", "subcategory": "Leite"},
        {"category": "Bebidas", "subcategory": "Leite"}
      ]
    }
  ]
}

CRITICAL: If you cannot extract the required fields (rawName, nameOptions and totalPrice) for any items, return an error:
{
  "error": "Could not extract required item information (name and price) from the receipt"
}`, itemMappingsText, categoriesText)

	resp, err := client.Models.GenerateContent(ctx, modelName, []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: prompt},
				{InlineData: &genai.Blob{
					MIMEType: "image/jpeg",
					Data:     imageData,
				}},
			},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from Gemini")
	}

	textPart := resp.Candidates[0].Content.Parts[0]
	if textPart.Text == "" {
		return nil, fmt.Errorf("unexpected response format from Gemini")
	}

	responseText := textPart.Text

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

func buildItemMappingsText(mappings []ItemMapping) string {
	if len(mappings) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("MAPEAMENTOS APRENDIDOS DE NOTAS FISCAIS ANTERIORES DO USUÁRIO:\n")
	builder.WriteString("Use estes mapeamentos para melhorar os nomes dos produtos quando o rawName for similar:\n\n")

	// Deduplicate mappings
	seen := make(map[string]string)
	for _, mapping := range mappings {
		if mapping.RawName != "" && mapping.Name != "" && mapping.RawName != mapping.Name {
			seen[mapping.RawName] = mapping.Name
		}
	}

	for rawName, improvedName := range seen {
		builder.WriteString(fmt.Sprintf("- \"%s\" → \"%s\"\n", rawName, improvedName))
	}

	builder.WriteString("\n")
	return builder.String()
}
