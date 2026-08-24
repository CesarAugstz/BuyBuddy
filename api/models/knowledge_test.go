package models

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestKnowledgeAttributesRoundTrip(t *testing.T) {
	original := KnowledgeAttributes{
		"product": "Milk",
		"rating":  float64(5),
		"nested": map[string]interface{}{
			"recommended": true,
		},
	}
	value, err := original.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	var decoded KnowledgeAttributes
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if !reflect.DeepEqual(decoded, original) {
		t.Fatalf("decoded = %#v, want %#v", decoded, original)
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(encoded) == "{}" {
		t.Fatal("attributes unexpectedly lost arbitrary keys")
	}
}

func TestKnowledgeAttributesNilUsesEmptyObject(t *testing.T) {
	var attributes KnowledgeAttributes
	value, err := attributes.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}
	if value != "{}" {
		t.Fatalf("Value() = %q, want {}", value)
	}

	if err := (&attributes).Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error = %v", err)
	}
	if attributes == nil || len(attributes) != 0 {
		t.Fatalf("Scan(nil) = %#v, want non-nil empty map", attributes)
	}
}

func TestKnowledgeKindsRemainArbitraryText(t *testing.T) {
	entry := KnowledgeEntry{Kind: "travel_recommendation"}
	if entry.Kind != "travel_recommendation" {
		t.Fatalf("Kind = %q, want arbitrary text preserved", entry.Kind)
	}
}

func TestKnowledgeEntryRevisionJSONIncludesRestorableFields(t *testing.T) {
	occurredAt := time.Date(2026, 8, 24, 12, 30, 0, 0, time.UTC)
	revision := KnowledgeEntryRevision{
		Kind:       "preference",
		OccurredAt: &occurredAt,
	}
	encoded, err := json.Marshal(revision)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded["kind"] != "preference" {
		t.Fatalf("revision kind = %#v, want preference", decoded["kind"])
	}
	if decoded["occurredAt"] != occurredAt.Format(time.RFC3339) {
		t.Fatalf("revision occurredAt = %#v, want %s", decoded["occurredAt"], occurredAt.Format(time.RFC3339))
	}
}
