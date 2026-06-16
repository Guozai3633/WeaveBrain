package repository

import (
	"context"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type projectRepository struct {
	db Conn
}

func NewProjectRepository(db Conn) ProjectRepository {
	return &projectRepository{db: db}
}

func (r *projectRepository) Create(ctx context.Context, p *entity.Project) error {
	query := `
		INSERT INTO projects (user_id, name, default_project, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`
	return r.db.QueryRow(ctx, query, p.UserID, p.Name, p.DefaultProject, p.CreatedAt, p.UpdatedAt).Scan(&p.ID)
}

func (r *projectRepository) GetByID(ctx context.Context, id int64) (*entity.Project, error) {
	p := &entity.Project{}
	query := `SELECT id, user_id, name, default_project, created_at, updated_at FROM projects WHERE id = $1 AND deleted_at IS NULL`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.UserID, &p.Name, &p.DefaultProject, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *projectRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Project, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, name, default_project, created_at, updated_at FROM projects
		 WHERE user_id = $1 AND deleted_at IS NULL ORDER BY default_project DESC, created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*entity.Project
	for rows.Next() {
		p := &entity.Project{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.DefaultProject, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

func (r *projectRepository) Update(ctx context.Context, p *entity.Project) error {
	query := `
		UPDATE projects SET name = $1, default_project = $2, updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
	`
	_, err := r.db.Exec(ctx, query, p.Name, p.DefaultProject, time.Now(), p.ID)
	return err
}

func (r *projectRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "UPDATE projects SET deleted_at = $1 WHERE id = $2", time.Now(), id)
	return err
}

func (r *projectRepository) List(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.Project, int64, error) {
	projects, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	var count int64 = int64(len(projects))
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit
	if offset >= len(projects) {
		return []*entity.Project{}, count, nil
	}
	end := offset + limit
	if end > len(projects) {
		end = len(projects)
	}
	return projects[offset:end], count, nil
}
