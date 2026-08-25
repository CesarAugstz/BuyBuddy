package models

import "testing"

func TestLegacyChatMessagesDefaultToReceiptDomain(t *testing.T) {
	message := &ChatMessage{}
	if err := message.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if message.Domain != ChatDomainReceipt {
		t.Fatalf("legacy message domain = %q, want %q", message.Domain, ChatDomainReceipt)
	}
	if got := NormalizeChatDomain("unexpected"); got != ChatDomainReceipt {
		t.Fatalf("NormalizeChatDomain(unexpected) = %q, want receipt", got)
	}
	if got := NormalizeChatDomain(ChatDomainKnowledge); got != ChatDomainKnowledge {
		t.Fatalf("NormalizeChatDomain(knowledge) = %q, want knowledge", got)
	}
}
