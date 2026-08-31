package repository

import (
	"context"
	"fmt"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type TimelineRepository interface {
	GetTimeline(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.TimelineEvent, int64, error)
}

type timelineRepository struct {
	db Conn
}

func NewTimelineRepository(db Conn) TimelineRepository {
	return &timelineRepository{db: db}
}

func (r *timelineRepository) GetTimeline(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.TimelineEvent, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	// We use UNION ALL to fetch from both ideas and mcp_audit_logs.
	// For count:
	countQuery := `
		SELECT
			(SELECT COUNT(*) FROM ideas WHERE user_id = $1 AND deleted_at IS NULL) +
			(SELECT COUNT(*) FROM mcp_audit_logs WHERE user_id = $1 AND success = true AND tool_name != 'get_environment_context' AND tool_name != 'get_user_profile')
	`
	var total int64
	if err := r.db.QueryRow(ctx, countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT 
			'idea_' || id as global_id,
			'IDEA_CREATED' as event_type, 
			id::text as source_id, 
			'' as workflow_id,
			'' as correlation_id,
			'记录了灵感' as title, 
			raw_input as content, 
			'success' as status, 
			'idea' as icon_type, 
			created_at as timestamp
		FROM ideas
		WHERE user_id = $1 AND deleted_at IS NULL

		UNION ALL

		SELECT 
			'mcp_' || id as global_id,
			'TOOL_CALLED' as event_type, 
			id::text as source_id, 
			'' as workflow_id,
			'' as correlation_id,
			'调用了 ' || server_name as title, 
			'工具: ' || tool_name as content, 
			'success' as status, 
			server_name as icon_type, 
			created_at as timestamp
		FROM mcp_audit_logs
		WHERE user_id = $1 AND success = true 
		  AND tool_name != 'get_environment_context' 
		  AND tool_name != 'get_user_profile'
		  AND tool_name != 'query_ideas'

		ORDER BY timestamp DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []*entity.TimelineEvent
	for rows.Next() {
		ev := &entity.TimelineEvent{UserID: userID}
		if err := rows.Scan(
			&ev.ID,
			&ev.Type,
			&ev.SourceID,
			&ev.WorkflowID,
			&ev.CorrelationID,
			&ev.Title,
			&ev.Content,
			&ev.Status,
			&ev.IconType,
			&ev.Timestamp,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan timeline event: %w", err)
		}
		events = append(events, ev)
	}

	return events, total, nil
}
