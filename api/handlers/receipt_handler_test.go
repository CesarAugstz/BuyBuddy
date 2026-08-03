package handlers

import "testing"

func TestParseReceiptDate(t *testing.T) {
	for _, value := range []string{
		"2026-08-03T14:30:00Z",
		"2026-08-03T14:30:00-03:00",
		"2026-08-03T14:30:00",
		"2026-08-03 14:30:00",
		"2026-08-03",
	} {
		if _, err := parseReceiptDate(value); err != nil {
			t.Errorf("parseReceiptDate(%q) error = %v", value, err)
		}
	}

	if _, err := parseReceiptDate("not-a-date"); err == nil {
		t.Error("parseReceiptDate() accepted an invalid date")
	}
}

func TestBuildReceiptItemPreservesAllFields(t *testing.T) {
	item := buildReceiptItem(map[string]interface{}{
		"rawName":         "CAFE TESTE",
		"nameOptions":     []interface{}{"Cafe Teste"},
		"brand":           "Teste",
		"quantity":        2.0,
		"unit":            "un",
		"unitPrice":       10.5,
		"grossTotalPrice": 22.0,
		"discount":        1.0,
		"totalPrice":      21.0,
		"barcode":         "7891234567890",
	})

	if item.RawName != "CAFE TESTE" ||
		item.Name != "Cafe Teste" ||
		item.Brand != "Teste" ||
		item.Quantity != 2 ||
		item.Unit != "un" ||
		item.UnitPrice != 10.5 ||
		item.GrossTotalPrice != 22 ||
		item.Discount != 1 ||
		item.TotalPrice != 21 ||
		item.Barcode != "7891234567890" {
		t.Errorf("buildReceiptItem() dropped data: %+v", item)
	}
}

func TestBuildReceiptItemDefaultsGrossPriceToNetPlusDiscount(t *testing.T) {
	item := buildReceiptItem(map[string]interface{}{
		"rawName":    "CAFE TESTE",
		"totalPrice": 8.0,
		"discount":   2.0,
	})

	if item.GrossTotalPrice != 10 {
		t.Errorf("GrossTotalPrice = %v, want 10", item.GrossTotalPrice)
	}
}
