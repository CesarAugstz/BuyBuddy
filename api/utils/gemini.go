package utils

import (
	"buybuddy-api/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"

	"google.golang.org/genai"
)

type ReceiptData struct {
	Company           string                   `json:"company"`
	Date              string                   `json:"date"`
	Total             float64                  `json:"total"`
	ReceiptDiscount   float64                  `json:"receiptDiscount"`
	AdditionalCharges float64                  `json:"additionalCharges"`
	AccessKey         string                   `json:"accessKey"`
	Items             []map[string]interface{} `json:"items"`
}

type receiptCorrection struct {
	ReceiptDiscount   *float64                `json:"receiptDiscount"`
	AdditionalCharges *float64                `json:"additionalCharges"`
	Items             []receiptItemCorrection `json:"items"`
}

type receiptItemCorrection struct {
	Index           *int     `json:"index"`
	Quantity        *float64 `json:"quantity"`
	GrossTotalPrice *float64 `json:"grossTotalPrice"`
	Discount        *float64 `json:"discount"`
	TotalPrice      *float64 `json:"totalPrice"`
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

	if !models.IsSupportedGeminiModel(modelName) {
		modelName = models.DefaultReceiptModel
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
- Linhas como "DESCONTO", "DESC", "PROMOÇÃO", "OFERTA" ou valores negativos logo após um produto normalmente pertencem ao produto anterior; NÃO crie um item separado para a linha de desconto
- Quando houver desconto de produto, grossTotalPrice é o valor antes do desconto, discount é um valor positivo e totalPrice é o valor final líquido: grossTotalPrice - discount
- Se não houver desconto, use discount = 0 e grossTotalPrice = totalPrice
- Agrupe linhas do mesmo produto quando rawName, unidade e preço bruto unitário forem iguais, mesmo que apenas algumas unidades tenham desconto
- Para produtos agrupados, some quantity, grossTotalPrice, discount e totalPrice
- unitPrice SEMPRE representa o preço líquido médio realmente pago por unidade: totalPrice / quantity. Ele deve considerar todos os descontos do grupo
- receiptDiscount representa apenas descontos globais no total da nota que não pertencem claramente a um produto
- additionalCharges representa taxas, acréscimos, entrega ou outros valores globais cobrados fora dos itens
- Confira a fórmula antes de responder: soma(totalPrice) - receiptDiscount + additionalCharges = total

Para cada item, você DEVE extrair no mínimo:
- rawName: O nome EXATO do produto como escrito na nota fiscal, incluindo abreviações (OBRIGATÓRIO)
- nameOptions: Um array de 1-3 versões MELHORADAS e legíveis do nome do produto - expanda abreviações, corrija erros, deixe claro. A primeira opção deve ser a mais provável, seguida de alternativas se aplicável (OBRIGATÓRIO)
- totalPrice: Preço total deste item (OBRIGATÓRIO)

Adicionalmente, extraia se visível:
- brand: Nome da marca (se visível, caso contrário null)
- quantity: Quantidade numérica
- unit: Unidade de medida ("kg", "un", "L", "g", "ml", "cx" para caixa, etc.)
- unitPrice: Preço líquido médio realmente pago por unidade após descontos (totalPrice/quantity)
- grossTotalPrice: Preço total do item antes de descontos
- discount: Desconto total aplicado ao item como número positivo; use 0 quando não houver
- categoryOptions: Array de 1-2 possíveis categorias com suas subcategorias em PORTUGUÊS. A primeira deve ser a mais provável. Formato: [{"category": "Alimentos", "subcategory": "Laticínios"}]
- Se algum campo de um item estiver ilegível, preserve os campos legíveis, use null apenas no campo incerto e adicione "needsReview": true e "warning": "motivo"
- Nunca descarte toda a nota porque um único item está incerto

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
  "receiptDiscount": 0.00,
  "additionalCharges": 0.00,
  "accessKey": "44-digit number or null",
  "items": [
    {
      "rawName": "LT UHT ITAMBE",
      "nameOptions": ["Leite UHT Itambé", "Leite Longa Vida Itambé"],
      "brand": "Itambé",
      "quantity": 1.0,
      "unit": "un",
      "unitPrice": 0.00,
      "grossTotalPrice": 0.00,
      "discount": 0.00,
      "totalPrice": 0.00,
      "needsReview": false,
      "warning": null,
      "categoryOptions": [
        {"category": "Laticínios", "subcategory": "Leite"},
        {"category": "Bebidas", "subcategory": "Leite"}
      ]
    }
  ]
}
}`, itemMappingsText, categoriesText)

	initialContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: prompt},
			{InlineData: &genai.Blob{
				MIMEType: "image/jpeg",
				Data:     imageData,
			}},
		},
	}
	config := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr[float32](0),
		ResponseMIMEType: "application/json",
	}

	resp, err := client.Models.GenerateContent(
		ctx,
		modelName,
		[]*genai.Content{initialContent},
		config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	receiptData, err := parseGeminiReceiptResponse(resp.Text())
	if err != nil {
		return nil, err
	}

	calculatedTotal, matches := reconcileReceiptTotal(receiptData)
	if matches {
		normalizeReceiptItems(receiptData)
		return receiptData, nil
	}

	log.Printf(
		"Receipt reconciliation required: calculated=%.2f receiptTotal=%.2f difference=%.2f",
		calculatedTotal,
		receiptData.Total,
		receiptData.Total-calculatedTotal,
	)

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("calculated receipt total %.2f does not match receipt total %.2f", calculatedTotal, receiptData.Total)
	}

	modelContent := resp.Candidates[0].Content
	modelContent.Role = "model"
	correction := &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: buildReconciliationPrompt(calculatedTotal, receiptData.Total)}},
	}
	correctionConfig := &genai.GenerateContentConfig{
		Temperature:        genai.Ptr[float32](0),
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: receiptCorrectionSchema(),
	}
	correctedResp, err := client.Models.GenerateContent(
		ctx,
		modelName,
		[]*genai.Content{initialContent, modelContent, correction},
		correctionConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile receipt totals: %w", err)
	}

	receiptCorrection, err := parseReceiptCorrection(correctedResp.Text())
	if err != nil {
		return nil, fmt.Errorf("failed to parse receipt correction: %w", err)
	}
	if err := applyReceiptCorrection(receiptData, receiptCorrection); err != nil {
		return nil, fmt.Errorf("failed to apply receipt correction: %w", err)
	}

	correctedTotal, matches := reconcileReceiptTotal(receiptData)
	if !matches {
		return nil, fmt.Errorf(
			"calculated receipt total %.2f does not match receipt total %.2f after reconciliation",
			correctedTotal,
			receiptData.Total,
		)
	}

	normalizeReceiptItems(receiptData)
	return receiptData, nil
}

func normalizeReceiptItems(receipt *ReceiptData) {
	grouped := make(map[string]map[string]interface{}, len(receipt.Items))
	keys := make([]string, 0, len(receipt.Items))

	for index, original := range receipt.Items {
		item := make(map[string]interface{}, len(original))
		for key, value := range original {
			item[key] = value
		}

		totalPrice, hasTotal := numberFromMap(item, "totalPrice")
		quantity, hasQuantity := numberFromMap(item, "quantity")
		if !hasQuantity || quantity <= 0 {
			quantity = 1
		}

		discount, hasDiscount := numberFromMap(item, "discount")
		if !hasDiscount {
			discount = 0
		}

		grossTotalPrice, hasGrossTotal := numberFromMap(item, "grossTotalPrice")
		if !hasGrossTotal && hasTotal {
			grossTotalPrice = totalPrice + discount
		}

		unit := normalizedItemText(item["unit"])
		if unit == "" {
			unit = "UN"
			item["unit"] = "un"
		}

		description := normalizedItemText(item["rawName"])
		if description == "" {
			description = normalizedItemText(item["name"])
		}

		groupKey := fmt.Sprintf("unmatched-%d", index)
		if description != "" && hasTotal && grossTotalPrice >= 0 {
			grossUnitPriceCents := int64(math.Round((grossTotalPrice / quantity) * 100))
			groupKey = fmt.Sprintf("%s\x00%s\x00%d", description, unit, grossUnitPriceCents)
		}

		item["quantity"] = quantity
		item["grossTotalPrice"] = roundCurrency(grossTotalPrice)
		item["discount"] = roundCurrency(discount)
		if hasTotal {
			item["totalPrice"] = roundCurrency(totalPrice)
		}

		existing, found := grouped[groupKey]
		if !found {
			grouped[groupKey] = item
			keys = append(keys, groupKey)
			continue
		}

		existingQuantity, _ := numberFromMap(existing, "quantity")
		existingGrossTotal, _ := numberFromMap(existing, "grossTotalPrice")
		existingDiscount, _ := numberFromMap(existing, "discount")
		existingTotal, _ := numberFromMap(existing, "totalPrice")

		existing["quantity"] = existingQuantity + quantity
		existing["grossTotalPrice"] = roundCurrency(existingGrossTotal + grossTotalPrice)
		existing["discount"] = roundCurrency(existingDiscount + discount)
		existing["totalPrice"] = roundCurrency(existingTotal + totalPrice)
	}

	receipt.Items = make([]map[string]interface{}, 0, len(keys))
	for _, key := range keys {
		item := grouped[key]
		quantity, _ := numberFromMap(item, "quantity")
		totalPrice, _ := numberFromMap(item, "totalPrice")
		item["unitPrice"] = totalPrice / quantity
		receipt.Items = append(receipt.Items, item)
	}
}

func normalizedItemText(value interface{}) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.ToUpper(strings.Join(strings.Fields(text), " "))
}

func roundCurrency(value float64) float64 {
	return float64(int64(math.Round(value*100))) / 100
}

func reconcileReceiptTotal(receipt *ReceiptData) (float64, bool) {
	var itemTotalCents int64
	for _, item := range receipt.Items {
		totalPrice, ok := numberFromMap(item, "totalPrice")
		if !ok {
			return float64(itemTotalCents) / 100, false
		}
		itemTotalCents += int64(math.Round(totalPrice * 100))
	}

	itemTotalCents -= int64(math.Round(receipt.ReceiptDiscount * 100))
	itemTotalCents += int64(math.Round(receipt.AdditionalCharges * 100))
	receiptTotalCents := int64(math.Round(receipt.Total * 100))
	return float64(itemTotalCents) / 100, itemTotalCents == receiptTotalCents
}

func numberFromMap(values map[string]interface{}, key string) (float64, bool) {
	value, ok := values[key]
	if !ok {
		return 0, false
	}

	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case json.Number:
		parsed, err := number.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func buildReconciliationPrompt(itemTotal, receiptTotal float64) string {
	return fmt.Sprintf(`A validação matemática falhou: o total calculado foi %.2f, mas o total impresso na nota é %.2f.
Revise SOMENTE os campos monetários da imagem e da resposta anterior. Preserve nomes, marcas, categorias, data e demais dados já extraídos.
Verifique descontos próximos aos produtos, descontos globais no total da nota e taxas ou acréscimos.
Não invente valores nem altere o total impresso apenas para forçar a soma.

Retorne APENAS um patch JSON neste formato:
{
  "receiptDiscount": 0.00,
  "additionalCharges": 0.00,
  "items": [
    {
      "index": 0,
      "quantity": 1.0,
      "grossTotalPrice": 0.00,
      "discount": 0.00,
      "totalPrice": 0.00
    }
  ]
}

Inclua em items somente índices que precisam de correção. Os índices são baseados na ordem da sua resposta anterior, começando em zero.
receiptDiscount é somente desconto global não atribuído a produto. additionalCharges é somente taxa ou acréscimo global.
A fórmula final deve ser: soma(totalPrice) - receiptDiscount + additionalCharges = %.2f.
Nunca retorne um erro global e nunca repita o JSON completo da nota.`, itemTotal, receiptTotal, receiptTotal)
}

func parseReceiptCorrection(responseText string) (*receiptCorrection, error) {
	var correction receiptCorrection
	cleanJSON := extractJSON(responseText)
	if err := json.Unmarshal([]byte(cleanJSON), &correction); err != nil {
		return nil, fmt.Errorf("failed to parse correction JSON: %w", err)
	}

	if correction.ReceiptDiscount == nil &&
		correction.AdditionalCharges == nil &&
		len(correction.Items) == 0 {
		return nil, fmt.Errorf("Gemini returned an empty correction")
	}

	return &correction, nil
}

func receiptCorrectionSchema() map[string]interface{} {
	numberProperty := map[string]interface{}{"type": "number"}

	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"receiptDiscount":   numberProperty,
			"additionalCharges": numberProperty,
			"items": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"index":           map[string]interface{}{"type": "integer"},
						"quantity":        numberProperty,
						"grossTotalPrice": numberProperty,
						"discount":        numberProperty,
						"totalPrice":      numberProperty,
					},
					"required": []string{
						"index",
						"quantity",
						"grossTotalPrice",
						"discount",
						"totalPrice",
					},
				},
			},
		},
		"required": []string{"receiptDiscount", "additionalCharges", "items"},
	}
}

func applyReceiptCorrection(receipt *ReceiptData, correction *receiptCorrection) error {
	if correction.ReceiptDiscount != nil {
		receipt.ReceiptDiscount = roundCurrency(*correction.ReceiptDiscount)
	}
	if correction.AdditionalCharges != nil {
		receipt.AdditionalCharges = roundCurrency(*correction.AdditionalCharges)
	}

	for _, correctedItem := range correction.Items {
		if correctedItem.Index == nil {
			return fmt.Errorf("correction item is missing index")
		}
		index := *correctedItem.Index
		if index < 0 || index >= len(receipt.Items) {
			return fmt.Errorf("correction item index %d is out of range", index)
		}

		item := receipt.Items[index]
		if correctedItem.Quantity != nil {
			item["quantity"] = *correctedItem.Quantity
		}
		if correctedItem.GrossTotalPrice != nil {
			item["grossTotalPrice"] = roundCurrency(*correctedItem.GrossTotalPrice)
		}
		if correctedItem.Discount != nil {
			item["discount"] = roundCurrency(*correctedItem.Discount)
		}
		if correctedItem.TotalPrice != nil {
			item["totalPrice"] = roundCurrency(*correctedItem.TotalPrice)
		}
	}

	return nil
}

func parseGeminiReceiptResponse(responseText string) (*ReceiptData, error) {
	var result struct {
		Error             string                   `json:"error"`
		Company           *string                  `json:"company"`
		Date              *string                  `json:"date"`
		Total             *float64                 `json:"total"`
		ReceiptDiscount   *float64                 `json:"receiptDiscount"`
		AdditionalCharges *float64                 `json:"additionalCharges"`
		AccessKey         *string                  `json:"accessKey"`
		Items             []map[string]interface{} `json:"items"`
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

	if result.ReceiptDiscount != nil {
		receiptData.ReceiptDiscount = *result.ReceiptDiscount
	}

	if result.AdditionalCharges != nil {
		receiptData.AdditionalCharges = *result.AdditionalCharges
	}

	if result.Date != nil {
		receiptData.Date = *result.Date
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
