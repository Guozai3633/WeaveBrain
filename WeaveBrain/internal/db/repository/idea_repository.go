package repository

import (
	"context"
	"fmt"
	"time"

	"weavebrain/internal/entity"

	"github.com/pgvector/pgvector-go"
)

type ideaRepository struct {
	db Conn
}

func NewIdeaRepository(db Conn) IdeaRepository {
	return &ideaRepository{db: db}
}

func (r *ideaRepository) Create(ctx context.Context, i *entity.Idea) error {
	query := `
		INSERT INTO ideas (project_id, user_id, raw_input, structured_data, tags, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	return r.db.QueryRow(ctx, query,
		i.ProjectID, i.UserID, i.RawInput, i.StructuredData, i.Tags,
		time.Now(), time.Now(),
	).Scan(&i.ID)
}

func (r *ideaRepository) GetByID(ctx context.Context, id int64) (*entity.Idea, error) {
	i := &entity.Idea{}
	query := `
		SELECT id, project_id, user_id, raw_input, structured_data, tags, created_at, updated_at
		FROM ideas WHERE id = $1 AND deleted_at IS NULL
	`
	err := r.db.QueryRow(ctx, query, id).Scan(
		&i.ID, &i.ProjectID, &i.UserID, &i.RawInput, &i.StructuredData, &i.Tags,
		&i.CreatedAt, &i.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (r *ideaRepository) GetByProjectID(ctx context.Context, projectID int64, page, limit int) ([]*entity.Idea, int64, error) {
	offset := (page - 1) * limit

	var count int64
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM ideas WHERE project_id = $1 AND deleted_at IS NULL", projectID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, project_id, user_id, raw_input, structured_data, tags, created_at, updated_at
		 FROM ideas WHERE project_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ideas []*entity.Idea
	for rows.Next() {
		i := &entity.Idea{}
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.UserID, &i.RawInput, &i.StructuredData, &i.Tags, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, 0, err
		}
		ideas = append(ideas, i)
	}
	return ideas, count, nil
}

func (r *ideaRepository) Update(ctx context.Context, i *entity.Idea) error {
	query := `
		UPDATE ideas SET raw_input = $1, structured_data = $2, tags = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query,
		i.RawInput, i.StructuredData, i.Tags, time.Now(), i.ID,
	)
	return err
}

func (r *ideaRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, "UPDATE ideas SET deleted_at = $1 WHERE id = $2", time.Now(), id)
	return err
}

func (r *ideaRepository) ListByTags(ctx context.Context, tags []string, projectID int64) ([]*entity.Idea, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, project_id, user_id, raw_input, structured_data, tags, created_at, updated_at
		 FROM ideas WHERE project_id = $1 AND tags && $2 AND deleted_at IS NULL
		 ORDER BY created_at DESC`,
		projectID, tags,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ideas []*entity.Idea
	for rows.Next() {
		i := &entity.Idea{}
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.UserID, &i.RawInput, &i.StructuredData, &i.Tags, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		ideas = append(ideas, i)
	}
	return ideas, nil
}

func (r *ideaRepository) Search(ctx context.Context, projectID int64, query string, tags []string, limit, offset int) ([]*entity.Idea, int64, error) {
	where := "project_id = $1 AND deleted_at IS NULL"
	args := []any{projectID}
	argIdx := 2

	if query != "" {
		where += fmt.Sprintf(" AND raw_input ILIKE $%d", argIdx)
		args = append(args, "%"+query+"%")
		argIdx++
	}

	if len(tags) > 0 {
		where += fmt.Sprintf(" AND tags && $%d", argIdx)
		args = append(args, tags)
		argIdx++
	}

	var count int64
	err := r.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM ideas WHERE "+where, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	rows, err := r.db.Query(ctx,
		fmt.Sprintf(`SELECT id, project_id, user_id, raw_input, structured_data, tags, created_at, updated_at
		 FROM ideas WHERE %s
		 ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, where, argIdx, argIdx+1),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var ideas []*entity.Idea
	for rows.Next() {
		i := &entity.Idea{}
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.UserID, &i.RawInput, &i.StructuredData, &i.Tags, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, 0, err
		}
		ideas = append(ideas, i)
	}
	return ideas, count, nil
}

func (r *ideaRepository) SearchBySimilarity(ctx context.Context, projectID int64, embedding []float32, limit int) ([]*entity.Idea, error) {
	vec := pgvector.NewVector(embedding)
	rows, err := r.db.Query(ctx,
		`SELECT id, project_id, user_id, raw_input, structured_data, tags, created_at, updated_at
		 FROM ideas
		 WHERE project_id = $1 AND deleted_at IS NULL AND embedding IS NOT NULL
		 ORDER BY embedding <=> $2
		 LIMIT $3`,
		projectID, vec, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var similarIdeas []*entity.Idea
	for rows.Next() {
		i := &entity.Idea{}
		if err := rows.Scan(&i.ID, &i.ProjectID, &i.UserID, &i.RawInput, &i.StructuredData, &i.Tags, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		similarIdeas = append(similarIdeas, i)
	}
	return similarIdeas, nil
}

func (r *ideaRepository) UpdateEmbedding(ctx context.Context, ideaID int64, embedding []float32) error {
	vec := pgvector.NewVector(embedding)
	_, err := r.db.Exec(ctx,
		"UPDATE ideas SET embedding = $1 WHERE id = $2",
		vec, ideaID,
	)
	return err
}

func (r *ideaRepository) GetWithoutEmbedding(ctx context.Context, limit int) ([]*entity.Idea, error) {
	rows2, err := r.db.Query(ctx,
		`SELECT id, project_id, user_id, raw_input, structured_data, tags, created_at, updated_at
		 FROM ideas WHERE embedding IS NULL AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	var ideasWithout []*entity.Idea
	for rows2.Next() {
		i := &entity.Idea{}
		if err := rows2.Scan(&i.ID, &i.ProjectID, &i.UserID, &i.RawInput, &i.StructuredData, &i.Tags, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		ideasWithout = append(ideasWithout, i)
	}
	return ideasWithout, nil
}
