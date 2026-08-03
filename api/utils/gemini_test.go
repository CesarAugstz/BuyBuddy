package utils

import (
	"math"
	"strings"
	"testing"
)

func TestParseGeminiReceiptResponsePreservesAllFields(t *testing.T) {
	response := `{
		"company": "Mercado Teste",
		"date": "2026-08-03T14:30:00",
		"total": 42.75,
		"receiptDiscount": 1.25,
		"additionalCharges": 0.50,
		"accessKey": "12345678901234567890123456789012345678901234",
		"items": [{
			"rawName": "CAFE TESTE",
			"nameOptions": ["Cafe Teste"],
			"brand": "Teste",
			"quantity": 2,
			"unit": "un",
			"unitPrice": 10.5,
			"grossTotalPrice": 22,
			"discount": 1,
			"totalPrice": 21,
			"barcode": "7891234567890",
			"categoryOptions": [{"category": "Alimentos", "subcategory": "Mercearia"}]
		}]
	}`

	receipt, err := parseGeminiReceiptResponse(response)
	if err != nil {
		t.Fatalf("parseGeminiReceiptResponse() error = %v", err)
	}

	if receipt.Company != "Mercado Teste" {
		t.Errorf("Company = %q", receipt.Company)
	}
	if receipt.Date != "2026-08-03T14:30:00" {
		t.Errorf("Date = %q", receipt.Date)
	}
	if receipt.Total != 42.75 {
		t.Errorf("Total = %v", receipt.Total)
	}
	if receipt.ReceiptDiscount != 1.25 {
		t.Errorf("ReceiptDiscount = %v", receipt.ReceiptDiscount)
	}
	if receipt.AdditionalCharges != 0.50 {
		t.Errorf("AdditionalCharges = %v", receipt.AdditionalCharges)
	}
	if receipt.AccessKey != "12345678901234567890123456789012345678901234" {
		t.Errorf("AccessKey = %q", receipt.AccessKey)
	}
	if len(receipt.Items) != 1 {
		t.Fatalf("len(Items) = %d", len(receipt.Items))
	}

	item := receipt.Items[0]
	for _, field := range []string{
		"rawName",
		"nameOptions",
		"brand",
		"quantity",
		"unit",
		"unitPrice",
		"grossTotalPrice",
		"discount",
		"totalPrice",
		"barcode",
		"categoryOptions",
	} {
		if _, ok := item[field]; !ok {
			t.Errorf("item field %q was dropped", field)
		}
	}
}

func TestReconcileReceiptTotalUsesNetItemPricesInCents(t *testing.T) {
	receipt := &ReceiptData{
		Total: 19.98,
		Items: []map[string]interface{}{
			{
				"grossTotalPrice": 12.00,
				"discount":        2.01,
				"totalPrice":      9.99,
			},
			{
				"grossTotalPrice": 9.99,
				"discount":        0.00,
				"totalPrice":      9.99,
			},
		},
	}

	total, matches := reconcileReceiptTotal(receipt)
	if !matches || total != 19.98 {
		t.Errorf("reconcileReceiptTotal() = (%v, %v)", total, matches)
	}

	receipt.Total = 19.99
	if _, matches := reconcileReceiptTotal(receipt); matches {
		t.Error("reconcileReceiptTotal() accepted mismatched totals")
	}
}

func TestReconcileReceiptTotalIncludesGlobalAdjustments(t *testing.T) {
	receipt := &ReceiptData{
		Total:             18.50,
		ReceiptDiscount:   2.00,
		AdditionalCharges: 0.50,
		Items: []map[string]interface{}{
			{"totalPrice": 10.00},
			{"totalPrice": 10.00},
		},
	}

	total, matches := reconcileReceiptTotal(receipt)
	if !matches || total != 18.50 {
		t.Errorf("reconcileReceiptTotal() = (%v, %v)", total, matches)
	}
}

func TestBuildReconciliationPromptIncludesDiscrepancyAndDiscountInstructions(t *testing.T) {
	prompt := buildReconciliationPrompt(18.50, 17.25)

	for _, expected := range []string{"18.50", "17.25", "receiptDiscount", "additionalCharges", "index"} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt does not contain %q", expected)
		}
	}
}

func TestParseAndApplyReceiptCorrectionOnlyChangesPrices(t *testing.T) {
	receipt := &ReceiptData{
		Company: "Mercado",
		Items: []map[string]interface{}{
			{
				"rawName":         "CAFE",
				"quantity":        1.0,
				"grossTotalPrice": 10.0,
				"discount":        0.0,
				"totalPrice":      10.0,
			},
		},
	}

	correction, err := parseReceiptCorrection(`{
		"receiptDiscount": 1.00,
		"additionalCharges": 0.50,
		"items": [{
			"index": 0,
			"quantity": 2,
			"grossTotalPrice": 20,
			"discount": 2,
			"totalPrice": 18
		}]
	}`)
	if err != nil {
		t.Fatalf("parseReceiptCorrection() error = %v", err)
	}
	if err := applyReceiptCorrection(receipt, correction); err != nil {
		t.Fatalf("applyReceiptCorrection() error = %v", err)
	}

	if receipt.Company != "Mercado" || receipt.Items[0]["rawName"] != "CAFE" {
		t.Error("correction changed non-price receipt data")
	}
	if receipt.ReceiptDiscount != 1 || receipt.AdditionalCharges != 0.5 {
		t.Errorf("global adjustments = (%v, %v)", receipt.ReceiptDiscount, receipt.AdditionalCharges)
	}
	assertItemNumber(t, receipt.Items[0], "quantity", 2)
	assertItemNumber(t, receipt.Items[0], "grossTotalPrice", 20)
	assertItemNumber(t, receipt.Items[0], "discount", 2)
	assertItemNumber(t, receipt.Items[0], "totalPrice", 18)
}

func TestNormalizeReceiptItemsGroupsMatchingProductsAndAveragesPaidPrice(t *testing.T) {
	receipt := &ReceiptData{
		Items: []map[string]interface{}{
			{
				"rawName":         "CERV.EISENBAHN",
				"quantity":        1.0,
				"unit":            "un",
				"unitPrice":       4.79,
				"grossTotalPrice": 4.79,
				"discount":        0.0,
				"totalPrice":      4.79,
			},
			{
				"rawName":         " CERV.EISENBAHN ",
				"quantity":        1.0,
				"unit":            "UN",
				"unitPrice":       4.79,
				"grossTotalPrice": 4.79,
				"discount":        1.0,
				"totalPrice":      3.79,
			},
			{
				"rawName":         "CERV.EISENBAHN",
				"quantity":        1.0,
				"unit":            "un",
				"unitPrice":       4.79,
				"grossTotalPrice": 4.79,
				"discount":        0.0,
				"totalPrice":      4.79,
			},
		},
	}

	normalizeReceiptItems(receipt)

	if len(receipt.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(receipt.Items))
	}

	item := receipt.Items[0]
	assertItemNumber(t, item, "quantity", 3)
	assertItemNumber(t, item, "grossTotalPrice", 14.37)
	assertItemNumber(t, item, "discount", 1)
	assertItemNumber(t, item, "totalPrice", 13.37)
	assertItemNumber(t, item, "unitPrice", 13.37/3)
}

func TestNormalizeReceiptItemsKeepsDifferentGrossPricesSeparate(t *testing.T) {
	receipt := &ReceiptData{
		Items: []map[string]interface{}{
			{"rawName": "CAFE", "quantity": 1.0, "unit": "un", "grossTotalPrice": 4.79, "totalPrice": 4.79},
			{"rawName": "CAFE", "quantity": 1.0, "unit": "un", "grossTotalPrice": 5.79, "totalPrice": 5.79},
		},
	}

	normalizeReceiptItems(receipt)

	if len(receipt.Items) != 2 {
		t.Errorf("len(Items) = %d, want 2", len(receipt.Items))
	}
}

func assertItemNumber(t *testing.T, item map[string]interface{}, field string, want float64) {
	t.Helper()

	got, ok := numberFromMap(item, field)
	if !ok || math.Abs(got-want) > 0.000001 {
		t.Errorf("%s = %v, want %v", field, got, want)
	}
}
