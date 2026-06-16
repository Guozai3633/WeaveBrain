package repository

import (
	"context"
	"encoding/json"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type mcpAuditLogRepository struct {
	db Conn
}

func NewMCPAuditLogRepository(db Conn) MCPAuditLogRepository {
	return &mcpAuditLogRepository{db: db}
}

func (r *mcpAuditLogRepository) Create(ctx context.Context, log *entity.MCPAuditLog) error {
	inputJSON, err := json.Marshal(log.Input)
	if err != nil {
		inputJSON = []byte("{}")
	}

	var outputJSON []byte
	if log.Output != nil {
		outputJSON, err = json.Marshal(log.Output)
		if err != nil {
			outputJSON = nil
		}
	}

	query := `
		INSERT INTO mcp_audit_logs (user_id, tool_name, server_name, input, output, duration_ms, success, error_msg, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	return r.db.QueryRow(ctx, query,
		log.UserID, log.ToolName, log.ServerName, inputJSON, outputJSON,
		log.DurationMs, log.Success, log.ErrorMsg, log.CreatedAt,
	).Scan(&log.ID)
}

func (r *mcpAuditLogRepository) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.MCPAuditLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM mcp_audit_logs WHERE user_id = $1", userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, tool_name, server_name, input, output, duration_ms, success, error_msg, created_at
		 FROM mcp_audit_logs WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*entity.MCPAuditLog
	for rows.Next() {
		l := &entity.MCPAuditLog{}
		var inputBytes, outputBytes []byte
		if err := rows.Scan(
			&l.ID, &l.UserID, &l.ToolName, &l.ServerName,
			&inputBytes, &outputBytes,
			&l.DurationMs, &l.Success, &l.ErrorMsg, &l.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		if inputBytes != nil {
			json.Unmarshal(inputBytes, &l.Input)
		}
		if l.Input == nil {
			l.Input = make(map[string]any)
		}
		if outputBytes != nil {
			json.Unmarshal(outputBytes, &l.Output)
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}
