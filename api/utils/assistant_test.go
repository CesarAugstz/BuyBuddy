package utils

import (
	"buybuddy-api/models"
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
