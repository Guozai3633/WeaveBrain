package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// completionColumns is the shared SELECT column list for completion proposals.
const completionColumns = `
	id, user_id, capture_id, preview_id, source_revision, field_name,
	original_value, proposed_value, provenance, apply_policy,
	confidence, risk_level, evidence_spans, status, provider, model,
	config_version, accepted_by, accepted_at, created_at, updated_at`

type completionRepository struct {
	db Conn
}

// NewCompletionRepository creates a CompletionRepository backed by the given Conn.
func NewCompletionRepository(db Conn) CompletionRepository {
	return &completionRepository{db: db}
}

func (r *completionRepository) CreateProposals(
	ctx context.Context,
	userID, captureID uuid.UUID,
	proposals []*entity.CompletionProposal,
) error {
	if len(proposals) == 0 {
		return nil
	}
	for _, p := range proposals {
		spansJSON, err := json.Marshal(p.EvidenceSpans)
		if err != nil {
			return fmt.Errorf("encode evidence spans: %w", err)
		}
		p.UserID = userID
		p.CaptureID = captureID
		err = r.db.QueryRow(ctx, `
			INSERT INTO completion_proposals (
				user_id, capture_id, preview_id, source_revision, field_name,
				original_value, proposed_value, provenance, apply_policy,
				confidence, risk_level, evidence_spans, status, provider,
				model, config_version
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb,
			        $13, $14, $15, $16)
			RETURNING id, created_at, updated_at
		`,
			userID, captureID, p.PreviewID, p.SourceRevision, p.FieldName,
			p.OriginalValue, p.ProposedValue, string(p.Provenance), string(p.ApplyPolicy),
			p.Confidence, p.RiskLevel, spansJSON, string(p.Status),
			p.Provider, p.Model, p.ConfigVersion,
		).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
		if err != nil {
			return fmt.Errorf("insert completion proposal: %w", err)
		}
	}
	return nil
}

func (r *completionRepository) ListPendingByCapture(
	ctx context.Context,
	userID, captureID uuid.UUID,
) ([]*entity.CompletionProposal, error) {
	return r.query(ctx, `
		SELECT `+completionColumns+`
		FROM completion_proposals
		WHERE user_id = $1 AND capture_id = $2 AND status = 'pending'
		ORDER BY field_name
	`, userID, captureID)
}

func (r *completionRepository) ListByIDs(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) ([]*entity.CompletionProposal, error) {
	if len(ids) == 0 {
		return []*entity.CompletionProposal{}, nil
	}
	return r.query(ctx, `
		SELECT `+completionColumns+`
		FROM completion_proposals
		WHERE user_id = $1 AND id = ANY($2::uuid[])
		ORDER BY field_name
	`, userID, ids)
}

func (r *completionRepository) ExpireAllPending(
	ctx context.Context,
	userID, captureID uuid.UUID,
) error {
	_, err := r.db.Exec(ctx, `
		UPDATE completion_proposals
		SET status = 'expired', updated_at = now()
		WHERE user_id = $1 AND capture_id = $2 AND status = 'pending'
	`, userID, captureID)
	if err != nil {
		return fmt.Errorf("expire all pending completion proposals: %w", err)
	}
	return nil
}

func (r *completionRepository) MarkAccepted(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		UPDATE completion_proposals
		SET status = 'accepted', accepted_by = $3, accepted_at = now(), updated_at = now()
		WHERE user_id = $1 AND id = ANY($2::uuid[]) AND status = 'pending'
	`, userID, ids, userID)
	if err != nil {
		return fmt.Errorf("mark completion proposals accepted: %w", err)
	}
	return nil
}

func (r *completionRepository) MarkRejected(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		UPDATE completion_proposals
		SET status = 'rejected', updated_at = now()
		WHERE user_id = $1 AND id = ANY($2::uuid[]) AND status = 'pending'
	`, userID, ids)
	if err != nil {
		return fmt.Errorf("mark completion proposals rejected: %w", err)
	}
	return nil
}

func (r *completionRepository) MarkExpired(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `
		UPDATE completion_proposals
		SET status = 'expired', updated_at = now()
		WHERE user_id = $1 AND id = ANY($2::uuid[]) AND status = 'pending'
	`, userID, ids)
	if err != nil {
		return fmt.Errorf("mark completion proposals expired: %w", err)
	}
	return nil
}

func (r *completionRepository) query(
	ctx context.Context,
	query string,
	args ...any,
) ([]*entity.CompletionProposal, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query completion proposals: %w", err)
	}
	defer rows.Close()

	proposals := make([]*entity.CompletionProposal, 0)
	for rows.Next() {
		p, err := scanCompletionProposal(rows)
		if err != nil {
			return nil, err
		}
		proposals = append(proposals, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completion proposals: %w", err)
	}
	return proposals, nil
}

func scanCompletionProposal(row interface{ Scan(dest ...any) error }) (*entity.CompletionProposal, error) {
	p := &entity.CompletionProposal{}
	var (
		spansJSON  []byte
		confidence *float64
		riskLevel  *string
		acceptedBy *uuid.UUID
		acceptedAt *time.Time
	)
	if err := row.Scan(
		&p.ID, &p.UserID, &p.CaptureID, &p.PreviewID, &p.SourceRevision, &p.FieldName,
		&p.OriginalValue, &p.ProposedValue, &p.Provenance, &p.ApplyPolicy,
		&confidence, &riskLevel, &spansJSON, &p.Status, &p.Provider, &p.Model,
		&p.ConfigVersion, &acceptedBy, &acceptedAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan completion proposal: %w", err)
	}
	if len(spansJSON) > 0 {
		if err := json.Unmarshal(spansJSON, &p.EvidenceSpans); err != nil {
			return nil, fmt.Errorf("decode evidence spans: %w", err)
		}
	}
	p.Confidence = confidence
	if riskLevel != nil {
		p.RiskLevel = *riskLevel
	}
	p.AcceptedBy = acceptedBy
	p.AcceptedAt = acceptedAt
	return p, nil
}
