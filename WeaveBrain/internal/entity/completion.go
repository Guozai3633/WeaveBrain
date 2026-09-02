package entity

import (
	"time"

	"github.com/google/uuid"
)

// ProposalStatus is the lifecycle of a field completion proposal.
// pending proposals are immutable until they transition to a terminal state.
type ProposalStatus string

const (
	ProposalStatusPending  ProposalStatus = "pending"
	ProposalStatusAccepted ProposalStatus = "accepted"
	ProposalStatusRejected ProposalStatus = "rejected"
	ProposalStatusExpired  ProposalStatus = "expired"
)

// ApplyPolicy controls whether a proposal may be applied without per-field
// confirmation. safe_auto may be bulk-applied ("全部采用安全字段"); suggest_only
// requires explicit per-field confirmation; forbidden is never applied.
type ApplyPolicy string

const (
	ApplyPolicySafeAuto    ApplyPolicy = "safe_auto"
	ApplyPolicySuggestOnly ApplyPolicy = "suggest_only"
	ApplyPolicyForbidden   ApplyPolicy = "forbidden"
)

// Provenance records where a suggested value came from.
type Provenance string

const (
	ProvenanceAI        Provenance = "ai"
	ProvenanceInherited Provenance = "inherited"
	ProvenanceImport    Provenance = "import"
	ProvenanceUser      Provenance = "user"
	ProvenanceDevice    Provenance = "device"
)

// EvidenceSpan is a contiguous fragment of the original text supporting a
// factual suggestion. Start/End are byte offsets into the original text; a
// factual proposal without any evidence span is downgraded to suggest_only.
type EvidenceSpan struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Quote string `json:"quote"`
}

// CompletionProposal is one FIELD suggestion for a capture's missing MemoryCard
// field. A preview batch shares PreviewID; SourceRevision is the
// MemoryCard.Version the proposal is based on (optimistic-concurrency guard).
type CompletionProposal struct {
	ID             uuid.UUID      `json:"id"`
	UserID         uuid.UUID      `json:"user_id"`
	CaptureID      uuid.UUID      `json:"capture_id"`
	PreviewID      uuid.UUID      `json:"preview_id"`
	SourceRevision int64          `json:"source_revision"`
	FieldName      string         `json:"field_name"`
	OriginalValue  string         `json:"original_value"`
	ProposedValue  string         `json:"proposed_value"`
	Provenance     Provenance     `json:"provenance"`
	ApplyPolicy    ApplyPolicy    `json:"apply_policy"`
	Confidence     *float64       `json:"confidence,omitempty"`
	RiskLevel      string         `json:"risk_level,omitempty"`
	EvidenceSpans  []EvidenceSpan `json:"evidence_spans"`
	Status         ProposalStatus `json:"status"`
	Provider       string         `json:"provider,omitempty"`
	Model          string         `json:"model,omitempty"`
	ConfigVersion  string         `json:"config_version,omitempty"`
	AcceptedBy     *uuid.UUID     `json:"accepted_by,omitempty"`
	AcceptedAt     *time.Time     `json:"accepted_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// CompletionPreviewResult is the outcome of completion:preview. It never
// mutates the capture or card.
type CompletionPreviewResult struct {
	CaptureID      uuid.UUID             `json:"capture_id"`
	SourceRevision int64                 `json:"source_revision"`
	MissingFields  []string              `json:"missing_fields"`
	Proposals      []*CompletionProposal `json:"proposals"`
}

// CompletionApplyResult is the outcome of completion:apply / completion:undo.
type CompletionApplyResult struct {
	Card               *MemoryCard         `json:"memory_card"`
	Revision           *EnrichmentRevision `json:"revision,omitempty"`
	AppliedProposalIDs []uuid.UUID         `json:"applied_proposal_ids"`
}
