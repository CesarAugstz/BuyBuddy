package config

import "testing"

func TestKnowledgeOrganizerDisabledByDefaultAndParsesBoolean(t *testing.T) {
	t.Setenv("KNOWLEDGE_ORGANIZER_ENABLED", "")
	if Load().KnowledgeOrganizerEnabled {
		t.Fatal("organizer must be disabled by default for existing deployments")
	}
	t.Setenv("KNOWLEDGE_ORGANIZER_ENABLED", "true")
	if !Load().KnowledgeOrganizerEnabled {
		t.Fatal("organizer did not parse enabled feature flag")
	}
	t.Setenv("KNOWLEDGE_ORGANIZER_ENABLED", "not-a-boolean")
	if Load().KnowledgeOrganizerEnabled {
		t.Fatal("invalid organizer feature flag must use safe disabled default")
	}
}
