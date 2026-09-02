package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

var (
	// ErrCompletionDisabled reports that AI completion (or cloud text upload,
	// for preview) is not enabled for the user.
	ErrCompletionDisabled = errors.New("ai completion is disabled")
	// ErrCompletionNotReady reports that the memory card is not ready for AI
	// completion (it must be in the ready processing status).
	ErrCompletionNotReady = errors.New("memory card is not ready for ai completion")
	// ErrCompletionVersionConflict reports that the proposal is based on an
	// older card version and must be regenerated before applying.
	ErrCompletionVersionConflict = errors.New("completion proposal version conflict")
	// ErrNothingToUndo reports that there is no applied AI completion to undo.
	ErrNothingToUndo = errors.New("no ai completion to undo")
	// ErrCompletionLLM reports that the field-proposal generator failed.
	ErrCompletionLLM = errors.New("ai completion generation failed")
	// ErrInvalidCompletionInput reports malformed completion requests.
	ErrInvalidCompletionInput = errors.New("invalid completion input")
)

const (
	// completionLLMTimeout bounds a single preview's LLM call.
	completionLLMTimeout = 45 * time.Second
	// completionProvider is stamped on every proposal row.
	completionProvider = "ollama"
	// completionConfigVersion identifies the prompt + parser contract so rows
	// generated under a future schema are traceable.
	completionConfigVersion = "completion-prompt-v1"
)

// ApplyCompletionInput carries the proposals to apply and the card revision
// the client last saw (optimistic concurrency guard).
type ApplyCompletionInput struct {
	ProposalIDs    []uuid.UUID `json:"proposal_ids"`
	SourceRevision int64       `json:"source_revision"`
}

// CompletionService orchestrates AI field completion: preview (no mutation),
// apply (only empty fields), and undo (revert the last applied completion).
type CompletionService struct {
	captures  repository.CaptureRepository
	memory    repository.MemoryRepository
	settings  repository.UserAISettingsRepository
	proposals repository.CompletionRepository
	generator FieldProposalGenerator
}

// NewCompletionService creates a CompletionService. generator may be nil when
// the LLM is unavailable; Preview then returns ErrCompletionLLM while Apply and
// Undo (which never call the model) still work.
func NewCompletionService(
	captures repository.CaptureRepository,
	memory repository.MemoryRepository,
	settings repository.UserAISettingsRepository,
	proposals repository.CompletionRepository,
	generator FieldProposalGenerator,
) *CompletionService {
	return &CompletionService{
		captures:  captures,
		memory:    memory,
		settings:  settings,
		proposals: proposals,
		generator: generator,
	}
}

// Preview generates field proposals for a capture's missing MemoryCard fields.
// It never mutates the capture or card. The original text is sent to the LLM,
// so both AI completion and cloud-text upload must be enabled.
func (s *CompletionService) Preview(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CompletionPreviewResult, error) {
	if s == nil || s.captures == nil || s.settings == nil || s.proposals == nil {
		return nil, fmt.Errorf("completion service is not configured")
	}
	if userID == uuid.Nil || captureID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id and capture_id are required", ErrInvalidCompletionInput)
	}

	effective, err := s.effectiveSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !effective.AICompletionEnabled || !effective.CloudTextAllowed {
		return nil, ErrCompletionDisabled
	}

	existing, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, mapMemoryRepositoryError("load capture for completion preview", err)
	}
	if existing == nil || existing.MemoryCard == nil {
		return nil, ErrMemoryNotFound
	}
	card := existing.MemoryCard
	if card.ProcessingStatus != "ready" {
		return nil, ErrCompletionNotReady
	}

	missing := missingCardFields(card)

	// A new preview supersedes the previous pending batch.
	if err := s.proposals.ExpireAllPending(ctx, userID, captureID); err != nil {
		return nil, fmt.Errorf("expire stale completion proposals: %w", err)
	}

	result := &entity.CompletionPreviewResult{
		CaptureID:      captureID,
		SourceRevision: card.Version,
		MissingFields:  missing,
	}
	if len(missing) == 0 {
		return result, nil
	}
	if s.generator == nil {
		return nil, ErrCompletionLLM
	}

	var originalText string
	if existing.Capture != nil && existing.Capture.OriginalText != nil {
		originalText = *existing.Capture.OriginalText
	}
	if strings.TrimSpace(originalText) == "" {
		return result, nil
	}

	genCtx, cancel := context.WithTimeout(ctx, completionLLMTimeout)
	defer cancel()
	generated, err := s.generator.GenerateFieldProposals(genCtx, originalText, missing)
	if err != nil {
		return nil, ErrCompletionLLM
	}

	previewID := uuid.New()
	proposals := make([]*entity.CompletionProposal, 0, len(generated))
	for _, g := range generated {
		proposedValue, err := serializeProposedValue(g.FieldName, g.ProposedValue)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCompletionInput, err)
		}
		spans := buildEvidenceSpans(originalText, g.Evidence)
		policy := classifyApplyPolicy(g.FieldName, len(spans) > 0)
		risk := "medium"
		if len(spans) > 0 {
			risk = "low"
		}
		proposals = append(proposals, &entity.CompletionProposal{
			PreviewID:     previewID,
			SourceRevision: card.Version,
			FieldName:      g.FieldName,
			OriginalValue:  emptyStateOriginalValue(g.FieldName),
			ProposedValue:  proposedValue,
			Provenance:     entity.ProvenanceAI,
			ApplyPolicy:    policy,
			Confidence:     g.Confidence,
			RiskLevel:      risk,
			EvidenceSpans:  spans,
			Status:         entity.ProposalStatusPending,
			Provider:       completionProvider,
			Model:          s.generator.Model(),
			ConfigVersion:  completionConfigVersion,
		})
	}
	if err := s.proposals.CreateProposals(ctx, userID, captureID, proposals); err != nil {
		return nil, fmt.Errorf("persist completion proposals: %w", err)
	}
	result.Proposals = proposals
	return result, nil
}

// Apply fills the empty fields referenced by the given proposals. It is
// idempotent, version-safe, and never overwrites a non-empty (user/imported)
// field. apply does not transmit text, so only AI completion must be enabled.
func (s *CompletionService) Apply(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	input ApplyCompletionInput,
) (*entity.CompletionApplyResult, error) {
	if s == nil || s.captures == nil || s.memory == nil || s.settings == nil || s.proposals == nil {
		return nil, fmt.Errorf("completion service is not configured")
	}
	if userID == uuid.Nil || captureID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id and capture_id are required", ErrInvalidCompletionInput)
	}
	if len(input.ProposalIDs) == 0 {
		return nil, fmt.Errorf("%w: proposal_ids must not be empty", ErrInvalidCompletionInput)
	}

	effective, err := s.effectiveSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !effective.AICompletionEnabled {
		return nil, ErrCompletionDisabled
	}

	existing, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, mapMemoryRepositoryError("load capture for completion apply", err)
	}
	if existing == nil || existing.MemoryCard == nil {
		return nil, ErrMemoryNotFound
	}
	card := existing.MemoryCard
	if card.ProcessingStatus != "ready" {
		return nil, ErrCompletionNotReady
	}

	proposals, err := s.proposals.ListByIDs(ctx, userID, input.ProposalIDs)
	if err != nil {
		return nil, fmt.Errorf("load completion proposals: %w", err)
	}
	if len(proposals) != len(input.ProposalIDs) {
		return nil, ErrInvalidCompletionInput
	}
	for _, p := range proposals {
		if p.CaptureID != captureID {
			return nil, ErrInvalidCompletionInput
		}
	}

	// Idempotent no-op: every proposal is already accepted.
	allAccepted := true
	for _, p := range proposals {
		if p.Status != entity.ProposalStatusAccepted {
			allAccepted = false
			break
		}
	}
	if allAccepted {
		return &entity.CompletionApplyResult{Card: card, AppliedProposalIDs: input.ProposalIDs}, nil
	}

	// Version guard: a pending proposal based on an older card version is
	// expired and rejected (G6 pass criterion 5).
	var pending []*entity.CompletionProposal
	for _, p := range proposals {
		if p.Status != entity.ProposalStatusPending {
			continue
		}
		if p.SourceRevision != card.Version {
			if err := s.proposals.MarkExpired(ctx, userID, []uuid.UUID{p.ID}); err != nil {
				return nil, fmt.Errorf("expire stale completion proposal: %w", err)
			}
			return nil, ErrCompletionVersionConflict
		}
		pending = append(pending, p)
	}
	if len(pending) == 0 {
		return &entity.CompletionApplyResult{Card: card}, nil
	}

	// Fill only empty fields; non-empty (user/imported) values are protected.
	updated := copyMemoryCard(card)
	changes := map[string]any{}
	undo := map[string]any{}
	var appliedIDs []uuid.UUID
	var rejectedIDs []uuid.UUID
	for _, p := range pending {
		if !fieldEmpty(card, p.FieldName) {
			rejectedIDs = append(rejectedIDs, p.ID)
			continue
		}
		value, derr := decodeProposedValue(p.FieldName, p.ProposedValue)
		if derr != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCompletionInput, derr)
		}
		undo[p.FieldName] = currentFieldValue(card, p.FieldName)
		applyFieldValue(updated, p.FieldName, value)
		changes[p.FieldName] = value
		appliedIDs = append(appliedIDs, p.ID)
	}
	if len(rejectedIDs) > 0 {
		if err := s.proposals.MarkRejected(ctx, userID, rejectedIDs); err != nil {
			return nil, fmt.Errorf("reject protected-field completion proposals: %w", err)
		}
	}
	if len(appliedIDs) == 0 {
		return &entity.CompletionApplyResult{Card: card}, nil
	}

	previewID := proposals[0].PreviewID
	rev := &entity.EnrichmentRevision{
		UserID:         userID,
		CaptureID:      captureID,
		Source:         entity.EnrichmentSourceAI,
		SourceRevision: existing.Capture.Version,
		Changes:        changes,
		Provenance: map[string]any{
			"_completion": map[string]any{
				"preview_id": previewID.String(),
				"undo":       undo,
			},
		},
	}
	for field := range changes {
		rev.Provenance[field] = "completion"
	}

	revision, newCard, err := s.memory.AppendRevisionAndUpdateCard(ctx, userID, captureID, rev, updated)
	if err != nil {
		return nil, mapMemoryRepositoryError("apply completion", err)
	}
	if err := s.proposals.MarkAccepted(ctx, userID, appliedIDs); err != nil {
		return nil, fmt.Errorf("mark completion proposals accepted: %w", err)
	}

	return &entity.CompletionApplyResult{
		Card:               newCard,
		Revision:           revision,
		AppliedProposalIDs: appliedIDs,
	}, nil
}

// Undo reverts the most recent applied AI completion. It only reverts fields
// whose current value still equals what the AI set — fields the user edited
// afterwards are skipped. The proposals stay accepted (audit trail).
func (s *CompletionService) Undo(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CompletionApplyResult, error) {
	if s == nil || s.captures == nil || s.memory == nil {
		return nil, fmt.Errorf("completion service is not configured")
	}
	if userID == uuid.Nil || captureID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id and capture_id are required", ErrInvalidCompletionInput)
	}

	existing, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, mapMemoryRepositoryError("load capture for completion undo", err)
	}
	if existing == nil || existing.MemoryCard == nil {
		return nil, ErrMemoryNotFound
	}
	card := existing.MemoryCard

	revisions, err := s.memory.ListRevisions(ctx, userID, captureID)
	if err != nil {
		return nil, fmt.Errorf("load memory revisions for undo: %w", err)
	}

	var target *entity.EnrichmentRevision
	for _, rev := range revisions {
		if rev.Source != entity.EnrichmentSourceAI {
			continue
		}
		comp, ok := rev.Provenance["_completion"].(map[string]any)
		if ok && comp != nil {
			target = rev
			break
		}
	}
	if target == nil {
		return nil, ErrNothingToUndo
	}
	comp, _ := target.Provenance["_completion"].(map[string]any)
	undoMap, _ := comp["undo"].(map[string]any)
	if len(undoMap) == 0 {
		return nil, ErrNothingToUndo
	}

	updated := copyMemoryCard(card)
	reversed := map[string]any{}
	for field, orig := range undoMap {
		aiValue := target.Changes[field]
		if !fieldEqual(card, field, aiValue) {
			// The user edited this field after the AI apply — keep their value.
			continue
		}
		origValue, ok := decodeUndoValue(field, orig)
		if !ok {
			continue
		}
		applyFieldValue(updated, field, origValue)
		reversed[field] = origValue
	}
	if len(reversed) == 0 {
		return nil, ErrNothingToUndo
	}

	rev := &entity.EnrichmentRevision{
		UserID:         userID,
		CaptureID:      captureID,
		Source:         entity.EnrichmentSourceUser,
		SourceRevision: existing.Capture.Version,
		Changes:        reversed,
		Provenance:     map[string]any{"_undo": true},
	}
	for field := range reversed {
		rev.Provenance[field] = "undo"
	}

	revision, newCard, err := s.memory.AppendRevisionAndUpdateCard(ctx, userID, captureID, rev, updated)
	if err != nil {
		return nil, mapMemoryRepositoryError("undo completion", err)
	}
	return &entity.CompletionApplyResult{Card: newCard, Revision: revision}, nil
}

func (s *CompletionService) effectiveSettings(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error) {
	stored, err := s.settings.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load ai settings: %w", err)
	}
	if stored == nil {
		return entity.DefaultUserAISettings(userID), nil
	}
	return stored, nil
}

// missingCardFields returns the MemoryCard columns that are empty and therefore
// eligible for completion. primary_type "uncategorized" counts as missing
// because the fallback always fills it that way.
func missingCardFields(card *entity.MemoryCard) []string {
	var missing []string
	if strings.TrimSpace(card.Title) == "" {
		missing = append(missing, "title")
	}
	if card.Summary == nil || strings.TrimSpace(*card.Summary) == "" {
		missing = append(missing, "summary")
	}
	if card.PrimaryType == "" || card.PrimaryType == "uncategorized" {
		missing = append(missing, "primary_type")
	}
	if len(card.Tags) == 0 {
		missing = append(missing, "tags")
	}
	if len(card.KeyPoints) == 0 {
		missing = append(missing, "key_points")
	}
	return missing
}

// classifyApplyPolicy maps a field to its apply policy: the five completable
// fields are safe_auto when they carry evidence; without evidence they degrade
// to suggest_only; anything else is forbidden.
func classifyApplyPolicy(field string, hasEvidence bool) entity.ApplyPolicy {
	if !validCompletionFieldNames[field] {
		return entity.ApplyPolicyForbidden
	}
	if !hasEvidence {
		return entity.ApplyPolicySuggestOnly
	}
	return entity.ApplyPolicySafeAuto
}

// buildEvidenceSpans locates each quote verbatim in the original text. Only
// quotes found are kept (Start/End are byte offsets of the first occurrence).
func buildEvidenceSpans(originalText string, quotes []string) []entity.EvidenceSpan {
	spans := make([]entity.EvidenceSpan, 0, len(quotes))
	for _, q := range quotes {
		idx := strings.Index(originalText, q)
		if idx < 0 {
			continue
		}
		spans = append(spans, entity.EvidenceSpan{Start: idx, End: idx + len(q), Quote: q})
	}
	return spans
}

// emptyStateOriginalValue is the stored original_value for a missing field
// (used for the audit trail and for the protected-field check).
func emptyStateOriginalValue(field string) string {
	switch field {
	case "tags", "key_points":
		return "[]"
	case "primary_type":
		return "uncategorized"
	default:
		return ""
	}
}

// serializeProposedValue converts the generated value into the TEXT form stored
// on the proposal row (tags/key_points as a JSON array string).
func serializeProposedValue(field string, value any) (string, error) {
	switch field {
	case "tags", "key_points":
		items, ok := value.([]string)
		if !ok {
			return "", fmt.Errorf("%s must be an array", field)
		}
		b, err := json.Marshal(items)
		if err != nil {
			return "", fmt.Errorf("encode %s: %w", field, err)
		}
		return string(b), nil
	default:
		s, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("%s must be a string", field)
		}
		return s, nil
	}
}

// decodeProposedValue parses a stored proposal's TEXT value back into the
// field's typed form ([]string for tags/key_points, string otherwise).
func decodeProposedValue(field, raw string) (any, error) {
	switch field {
	case "tags", "key_points":
		var items []string
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			return nil, fmt.Errorf("decode %s proposal value: %w", field, err)
		}
		return items, nil
	default:
		return raw, nil
	}
}

// currentFieldValue returns a field's current value as a string or []string.
func currentFieldValue(card *entity.MemoryCard, field string) any {
	switch field {
	case "title":
		return card.Title
	case "summary":
		if card.Summary == nil {
			return ""
		}
		return *card.Summary
	case "primary_type":
		return card.PrimaryType
	case "tags":
		return card.Tags
	case "key_points":
		return card.KeyPoints
	default:
		return nil
	}
}

// fieldEmpty reports whether a card field is considered missing (empty).
// primary_type "uncategorized" counts as missing, mirroring missingCardFields.
func fieldEmpty(card *entity.MemoryCard, field string) bool {
	switch field {
	case "primary_type":
		return card.PrimaryType == "" || card.PrimaryType == "uncategorized"
	default:
		switch v := currentFieldValue(card, field).(type) {
		case string:
			return strings.TrimSpace(v) == ""
		case []string:
			return len(v) == 0
		default:
			return true
		}
	}
}

// fieldEqual reports whether the card's current field value equals the given
// value, tolerating JSON round-tripped []any slices from revision maps.
func fieldEqual(card *entity.MemoryCard, field string, value any) bool {
	current := currentFieldValue(card, field)
	switch want := value.(type) {
	case string:
		cur, ok := current.(string)
		return ok && cur == want
	case []string:
		cur, ok := current.([]string)
		return ok && stringSlicesEqual(cur, want)
	case []any:
		wantStrings := make([]string, 0, len(want))
		for _, item := range want {
			s, ok := item.(string)
			if !ok {
				return false
			}
			wantStrings = append(wantStrings, s)
		}
		cur, ok := current.([]string)
		return ok && stringSlicesEqual(cur, wantStrings)
	default:
		return false
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// applyFieldValue mutates the given card copy for one field.
func applyFieldValue(card *entity.MemoryCard, field string, value any) {
	switch field {
	case "title":
		if s, ok := value.(string); ok {
			card.Title = s
		}
	case "summary":
		if s, ok := value.(string); ok {
			if s == "" {
				card.Summary = nil
			} else {
				card.Summary = &s
			}
		}
	case "primary_type":
		if s, ok := value.(string); ok {
			card.PrimaryType = s
		}
	case "tags":
		if items, ok := value.([]string); ok {
			card.Tags = items
		}
	case "key_points":
		if items, ok := value.([]string); ok {
			card.KeyPoints = items
		}
	}
}

// decodeUndoValue rehydrates an undo map value into the field's typed form.
func decodeUndoValue(field string, orig any) (any, bool) {
	switch field {
	case "tags", "key_points":
		items, ok := orig.([]any)
		if !ok {
			return nil, false
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		s, ok := orig.(string)
		if !ok {
			return nil, false
		}
		return s, true
	}
}

// copyMemoryCard returns a shallow copy of the card with slice/heap fields
// deep-copied so mutating the copy never affects the stored card.
func copyMemoryCard(card *entity.MemoryCard) *entity.MemoryCard {
	if card == nil {
		return nil
	}
	c := *card
	if card.Summary != nil {
		s := *card.Summary
		c.Summary = &s
	}
	c.Tags = append([]string{}, card.Tags...)
	c.KeyPoints = append([]string{}, card.KeyPoints...)
	return &c
}
