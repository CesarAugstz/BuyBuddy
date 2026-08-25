package models

type AssistantQueryFilter struct {
	ProductName       []string `json:"productName,omitempty"`
	Company           []string `json:"company,omitempty"`
	Brand             []string `json:"brand,omitempty"`
	Category          []string `json:"category,omitempty"`
	Subcategory       []string `json:"subcategory,omitempty"`
	DateFrom          string   `json:"dateFrom,omitempty"`
	DateTo            string   `json:"dateTo,omitempty"`
	MinPrice          *float64 `json:"minPrice,omitempty"`
	MaxPrice          *float64 `json:"maxPrice,omitempty"`
	Limit             *int     `json:"limit,omitempty"`
	OrderBy           string   `json:"orderBy,omitempty"`
	ReturnFullReceipt bool     `json:"returnFullReceipt,omitempty"`
	ProductScope      string   `json:"productScope,omitempty"`
	ProductConcept    string   `json:"productConcept,omitempty"`
}

type ReceiptAssistantIntentResponse struct {
	Type       string                `json:"type"`
	Answer     string                `json:"answer,omitempty"`
	Specific   *AssistantQueryFilter `json:"specific,omitempty"`
	General    *AssistantQueryFilter `json:"general,omitempty"`
	Confidence string                `json:"confidence,omitempty"`
}

type KnowledgeAssistantIntentResponse struct {
	Type       string                    `json:"type"`
	Answer     string                    `json:"answer,omitempty"`
	Knowledge  *AssistantKnowledgeAction `json:"knowledge,omitempty"`
	Confidence string                    `json:"confidence,omitempty"`
}

type AssistantKnowledgeAction struct {
	Operation       string              `json:"operation"`
	EntryID         string              `json:"entryId,omitempty"`
	ExpectedVersion int                 `json:"expectedVersion,omitempty"`
	TopicID         string              `json:"topicId,omitempty"`
	Kind            string              `json:"kind,omitempty"`
	Title           string              `json:"title,omitempty"`
	Body            string              `json:"body,omitempty"`
	Attributes      KnowledgeAttributes `json:"attributes,omitempty"`
	Tags            []string            `json:"tags,omitempty"`
	OccurredAt      string              `json:"occurredAt,omitempty"`
	SearchQuery     string              `json:"searchQuery,omitempty"`
}

type AssistantSuggestionsResponse struct {
	Suggestions []string `json:"suggestions"`
}

type CompactReceiptItem struct {
	Name    string  `json:"n"`
	RawName string  `json:"rn"`
	Brand   string  `json:"b,omitempty"`
	Qty     float64 `json:"q"`
	Unit    string  `json:"u"`
	UP      float64 `json:"up"`
	TP      float64 `json:"tp"`
	Cat     string  `json:"cat,omitempty"`
	SubCat  string  `json:"sc,omitempty"`
	Barcode string  `json:"bc,omitempty"`
}

type CompactReceipt struct {
	ID      string               `json:"id"`
	Company string               `json:"co"`
	Date    string               `json:"d,omitempty"`
	Total   float64              `json:"t"`
	Items   []CompactReceiptItem `json:"items,omitempty"`
}

type CompactProductGroup struct {
	Name             string   `json:"name"`
	Brand            string   `json:"brand,omitempty"`
	Unit             string   `json:"unit,omitempty"`
	Variants         []string `json:"variants,omitempty"`
	Category         string   `json:"category,omitempty"`
	Subcategory      string   `json:"subcategory,omitempty"`
	PurchaseCount    int      `json:"purchaseCount"`
	TotalQuantity    float64  `json:"totalQuantity"`
	AverageUnitPrice float64  `json:"averageUnitPrice"`
	MinimumUnitPrice float64  `json:"minimumUnitPrice"`
	MaximumUnitPrice float64  `json:"maximumUnitPrice"`
	LatestUnitPrice  float64  `json:"latestUnitPrice"`
	LatestDate       string   `json:"latestDate,omitempty"`
}

type CompactReceiptResponse struct {
	Legend        map[string]string     `json:"_legend"`
	QueryStatus   string                `json:"queryStatus"`
	ProductScope  string                `json:"productScope,omitempty"`
	ProductGroups []CompactProductGroup `json:"productGroups,omitempty"`
	Receipts      []CompactReceipt      `json:"receipts"`
}
