package repository

import (
	"context"
	"time"

	"weavebrain/internal/entity"
)

type reminderRepository struct {
	db Conn
}

func NewReminderRepository(db Conn) ReminderRepository {
	return &reminderRepository{db: db}
}

func (r *reminderRepository) Create(ctx context.Context, rem *entity.Reminder) error {
	query := `
		INSERT INTO reminders (user_id, project_id, trigger_time, message, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	return r.db.QueryRow(ctx, query,
		rem.UserID, rem.ProjectID, rem.TriggerTime, rem.Message, rem.Status, rem.CreatedAt,
	).Scan(&rem.ID)
}

func (r *reminderRepository) GetByID(ctx context.Context, id int64) (*entity.Reminder, error) {
	rem := &entity.Reminder{}
	query := `SELECT id, user_id, project_id, trigger_time, message, status, created_at FROM reminders WHERE id = $1`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&rem.ID, &rem.UserID, &rem.ProjectID, &rem.TriggerTime, &rem.Message, &rem.Status, &rem.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return rem, nil
}

func (r *reminderRepository) GetPending(ctx context.Context, before time.Time, limit int) ([]*entity.Reminder, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, project_id, trigger_time, message, status, created_at
		 FROM reminders WHERE status = 'pending' AND trigger_time < $1
		 ORDER BY trigger_time ASC LIMIT $2`,
		before, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reminders []*entity.Reminder
	for rows.Next() {
		rem := &entity.Reminder{}
		if err := rows.Scan(&rem.ID, &rem.UserID, &rem.ProjectID, &rem.TriggerTime, &rem.Message, &rem.Status, &rem.CreatedAt); err != nil {
			return nil, err
		}
		reminders = append(reminders, rem)
	}
	return reminders, nil
}

func (r *reminderRepository) Update(ctx context.Context, rem *entity.Reminder) error {
	query := `UPDATE reminders SET status = $1 WHERE id = $2`
	_, err := r.db.Exec(ctx, query, rem.Status, rem.ID)
	return err
}

func (r *reminderRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "DELETE FROM reminders WHERE id = $1", id)
	return err
}
