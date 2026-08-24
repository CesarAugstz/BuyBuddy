package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

const (
	KnowledgeInboxName       = "Inbox"
	KnowledgeStatusActive    = "active"
	KnowledgeStatusMerged    = "merged"
	KnowledgeSourceManual    = "manual"
	KnowledgeSourceAssistant = "assistant"
	KnowledgeSourceOrganizer = "organizer"
)

type KnowledgeAttributes map[string]interface{}

func (a KnowledgeAttributes) Value() (driver.Value, error) {
	if a == nil {
		return "{}", nil
	}
	value, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("marshal knowledge attributes: %w", err)
	}
	return string(value), nil
}

func (a *KnowledgeAttributes) Scan(value interface{}) error {
	if value == nil {
		*a = KnowledgeAttributes{}
		return nil
	}

	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return fmt.Errorf("scan knowledge attributes: unsupported type %T", value)
	}

	if len(data) == 0 {
		*a = KnowledgeAttributes{}
		return nil
	}
	if err := json.Unmarshal(data, a); err != nil {
		return fmt.Errorf("scan knowledge attributes: %w", err)
	}
	return nil
}

type KnowledgeTopic struct {
	ID                     string          `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID                 string          `gorm:"type:uuid;not null;index:idx_knowledge_topics_user_parent_name,priority:1" json:"-"`
	User                   *User           `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	ParentID               *string         `gorm:"type:uuid;index:idx_knowledge_topics_user_parent_name,priority:2" json:"parentId,omitempty"`
	Parent                 *KnowledgeTopic `gorm:"foreignKey:ParentID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"-"`
	Name                   string          `gorm:"type:text;not null" json:"name"`
	NormalizedName         string          `gorm:"type:text;not null;index:idx_knowledge_topics_user_parent_name,priority:3" json:"-"`
	Description            string          `gorm:"type:text" json:"description,omitempty"`
	Depth                  int             `gorm:"not null" json:"depth"`
	PendingWriteCount      int             `gorm:"not null;default:0" json:"pendingWriteCount"`
	OrganizationDueAt      *time.Time      `gorm:"index:idx_knowledge_topics_organization,priority:1" json:"organizationDueAt,omitempty"`
	OrganizationLeaseUntil *time.Time      `gorm:"index:idx_knowledge_topics_organization,priority:2" json:"organizationLeaseUntil,omitempty"`
	LastOrganizedAt        *time.Time      `json:"lastOrganizedAt,omitempty"`
	CreatedAt              time.Time       `json:"createdAt"`
	UpdatedAt              time.Time       `json:"updatedAt"`
	DeletedAt              gorm.DeletedAt  `gorm:"index" json:"-"`
}

type KnowledgeEntry struct {
	ID         string              `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID     string              `gorm:"type:uuid;not null;index:idx_knowledge_entries_user_topic,priority:1;index:idx_knowledge_entries_user_kind_occurred,priority:1" json:"-"`
	User       *User               `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	TopicID    string              `gorm:"type:uuid;not null;index:idx_knowledge_entries_user_topic,priority:2" json:"topicId"`
	Topic      *KnowledgeTopic     `gorm:"foreignKey:TopicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT" json:"topic,omitempty"`
	Kind       string              `gorm:"type:text;not null;default:'note';index:idx_knowledge_entries_user_kind_occurred,priority:2" json:"kind"`
	Title      string              `gorm:"type:text;not null" json:"title"`
	Body       string              `gorm:"type:text;not null" json:"body"`
	Attributes KnowledgeAttributes `gorm:"type:jsonb;not null;default:'{}'" json:"attributes"`
	Tags       pq.StringArray      `gorm:"type:text[];not null;default:'{}'" json:"tags"`
	OccurredAt *time.Time          `gorm:"index:idx_knowledge_entries_user_kind_occurred,priority:3" json:"occurredAt,omitempty"`
	Source     string              `gorm:"type:text;not null" json:"source"`
	Status     string              `gorm:"type:text;not null;default:'active'" json:"status"`
	Version    int                 `gorm:"not null;default:1" json:"version"`
	CreatedAt  time.Time           `json:"createdAt"`
	UpdatedAt  time.Time           `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt      `gorm:"index" json:"-"`
}

type KnowledgeEntryRevision struct {
	ID             string              `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	EntryID        string              `gorm:"type:uuid;not null;index:idx_knowledge_revisions_entry_version,priority:1" json:"entryId"`
	UserID         string              `gorm:"type:uuid;not null;index" json:"-"`
	Version        int                 `gorm:"not null;index:idx_knowledge_revisions_entry_version,priority:2" json:"version"`
	TopicID        string              `gorm:"type:uuid;not null" json:"topicId"`
	Kind           string              `gorm:"type:text;not null;default:'note'" json:"kind"`
	Title          string              `gorm:"type:text;not null" json:"title"`
	Body           string              `gorm:"type:text;not null" json:"body"`
	Attributes     KnowledgeAttributes `gorm:"type:jsonb;not null" json:"attributes"`
	Tags           pq.StringArray      `gorm:"type:text[];not null" json:"tags"`
	OccurredAt     *time.Time          `json:"occurredAt,omitempty"`
	Status         string              `gorm:"type:text;not null" json:"status"`
	EntryDeletedAt *time.Time          `json:"-"`
	ChangedBy      string              `gorm:"type:text;not null" json:"changedBy"`
	Operation      string              `gorm:"type:text;not null" json:"operation"`
	CreatedAt      time.Time           `json:"createdAt"`
}

type KnowledgeTopicNode struct {
	KnowledgeTopic
	EntryCount int                  `json:"entryCount"`
	ChildCount int                  `json:"childCount"`
	Children   []KnowledgeTopicNode `json:"children"`
}

type KnowledgeTopicDetail struct {
	KnowledgeTopic
	EntryCount int              `json:"entryCount"`
	ChildCount int              `json:"childCount"`
	Breadcrumb []KnowledgeTopic `json:"breadcrumb"`
}

type KnowledgeSearchResult struct {
	Entry      KnowledgeEntry   `json:"entry"`
	Breadcrumb []KnowledgeTopic `json:"breadcrumb"`
}

type CreateKnowledgeTopicRequest struct {
	ParentID    *string `json:"parentId"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
}

type UpdateKnowledgeTopicRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	ParentID    *string `json:"parentId"`
	MoveToRoot  bool    `json:"moveToRoot"`
}

type CreateKnowledgeEntryRequest struct {
	TopicID    string              `json:"topicId"`
	Kind       string              `json:"kind"`
	Title      string              `json:"title"`
	Body       string              `json:"body"`
	Attributes KnowledgeAttributes `json:"attributes"`
	Tags       []string            `json:"tags"`
	OccurredAt *time.Time          `json:"occurredAt"`
}

type UpdateKnowledgeEntryRequest struct {
	ExpectedVersion   int                  `json:"expectedVersion"`
	TopicID           *string              `json:"topicId"`
	Kind              *string              `json:"kind"`
	Title             *string              `json:"title"`
	Body              *string              `json:"body"`
	Attributes        *KnowledgeAttributes `json:"attributes"`
	ReplaceAttributes bool                 `json:"replaceAttributes"`
	Tags              *[]string            `json:"tags"`
	OccurredAt        *time.Time           `json:"occurredAt"`
	ClearOccurredAt   bool                 `json:"clearOccurredAt"`
}

type UndoKnowledgeEntryRequest struct {
	ExpectedVersion int `json:"expectedVersion"`
}

type KnowledgeEntryMutation struct {
	TopicID           *string
	Kind              *string
	Title             *string
	Body              *string
	Attributes        *KnowledgeAttributes
	ReplaceAttributes bool
	Tags              *[]string
	OccurredAt        *time.Time
	ClearOccurredAt   bool
}

type KnowledgeSearchFilter struct {
	Query           string
	TopicID         string
	IncludeChildren bool
	Kind            string
	Tag             string
	OccurredFrom    *time.Time
	OccurredTo      *time.Time
	Limit           int
}

type KnowledgeAssistantTopic struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
}

type KnowledgeAssistantEntry struct {
	ID         string              `json:"id"`
	TopicID    string              `json:"topicId"`
	TopicPath  string              `json:"topicPath"`
	Kind       string              `json:"kind"`
	Title      string              `json:"title"`
	Body       string              `json:"body"`
	Attributes KnowledgeAttributes `json:"attributes"`
	Tags       []string            `json:"tags"`
	OccurredAt *time.Time          `json:"occurredAt,omitempty"`
	Version    int                 `json:"version"`
}

type KnowledgeAssistantContext struct {
	Topics  []KnowledgeAssistantTopic `json:"topics"`
	Entries []KnowledgeAssistantEntry `json:"recentEntries"`
}

type KnowledgeOrganizerTarget struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ParentPath        string `json:"parentPath,omitempty"`
	Description       string `json:"description,omitempty"`
	DirectEntryCount  int    `json:"directEntryCount"`
	PendingWriteCount int    `json:"pendingWriteCount"`
	Depth             int    `json:"depth"`
}

type KnowledgeOrganizerChild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	EntryCount  int    `json:"entryCount"`
	Depth       int    `json:"depth"`
}

type KnowledgeOrganizerEntry struct {
	ID         string              `json:"id"`
	TopicID    string              `json:"topicId"`
	Kind       string              `json:"kind"`
	Title      string              `json:"title"`
	Body       string              `json:"bodyExcerpt"`
	Tags       []string            `json:"tags"`
	Attributes KnowledgeAttributes `json:"attributes,omitempty"`
	OccurredAt *time.Time          `json:"occurredAt,omitempty"`
	Version    int                 `json:"version"`
	UpdatedAt  time.Time           `json:"updatedAt"`
}

type KnowledgeOrganizerRevision struct {
	EntryID   string    `json:"entryId"`
	TopicID   string    `json:"topicId"`
	Title     string    `json:"previousTitle"`
	Operation string    `json:"operation"`
	CreatedAt time.Time `json:"createdAt"`
}

type KnowledgeOrganizerContext struct {
	Target          KnowledgeOrganizerTarget     `json:"targetTopic"`
	Children        []KnowledgeOrganizerChild    `json:"childTopics"`
	RecentEntries   []KnowledgeOrganizerEntry    `json:"recentEntries"`
	Tags            []string                     `json:"existingTags"`
	SimilarEntries  []KnowledgeOrganizerEntry    `json:"similarOlderEntries"`
	RecentRevisions []KnowledgeOrganizerRevision `json:"recentOrganizerRevisions"`
}

type KnowledgeOrganizationOperation struct {
	Type                     string   `json:"type"`
	EntryID                  string   `json:"entryId,omitempty"`
	ExpectedVersion          int      `json:"expectedVersion,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	TemporaryID              string   `json:"temporaryId,omitempty"`
	Name                     string   `json:"name,omitempty"`
	Description              string   `json:"description,omitempty"`
	TargetTopicID            string   `json:"targetTopicId,omitempty"`
	Title                    string   `json:"title,omitempty"`
	CanonicalEntryID         string   `json:"canonicalEntryId,omitempty"`
	DuplicateEntryID         string   `json:"duplicateEntryId,omitempty"`
	CanonicalExpectedVersion int      `json:"canonicalExpectedVersion,omitempty"`
	DuplicateExpectedVersion int      `json:"duplicateExpectedVersion,omitempty"`
	Reason                   string   `json:"reason"`
}

type KnowledgeOrganizationPlan struct {
	Operations []KnowledgeOrganizationOperation `json:"operations"`
}

type KnowledgeOrganizationClaim struct {
	Topic                        KnowledgeTopic `json:"-"`
	LeaseUntil                   time.Time      `json:"-"`
	SyntheticManual              bool           `json:"-"`
	PendingWriteCountBeforeClaim int            `json:"-"`
	OrganizationDueAtBeforeClaim *time.Time     `json:"-"`
}

type KnowledgeOrganizationApplyResult struct {
	OperationsApplied int      `json:"operationsApplied"`
	ChangedEntryIDs   []string `json:"changedEntryIds"`
	CreatedTopicIDs   []string `json:"createdTopicIds"`
	AffectedTopicIDs  []string `json:"affectedTopicIds"`
}

type KnowledgeOrganizationResponse struct {
	Status string                           `json:"status"`
	Topic  KnowledgeTopic                   `json:"topic"`
	Result KnowledgeOrganizationApplyResult `json:"result"`
}
