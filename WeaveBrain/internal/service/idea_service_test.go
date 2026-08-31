package service

import (
	"context"
	"errors"
	"testing"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type fakeProjectRepo struct {
	project *entity.Project
	err     error
}

func (f *fakeProjectRepo) Create(context.Context, *entity.Project) error { panic("not used") }
func (f *fakeProjectRepo) GetByID(context.Context, int64) (*entity.Project, error) {
	return f.project, f.err
}
func (f *fakeProjectRepo) GetByUserID(context.Context, uuid.UUID) ([]*entity.Project, error) {
	panic("not used")
}
func (f *fakeProjectRepo) Update(context.Context, *entity.Project) error { panic("not used") }
func (f *fakeProjectRepo) Delete(context.Context, int64) error           { panic("not used") }
func (f *fakeProjectRepo) List(context.Context, uuid.UUID, int, int) ([]*entity.Project, int64, error) {
	panic("not used")
}

type fakeIdeaRepo struct {
	created           []*entity.Idea
	createErr         error
	listByProjectArgs []any
}

func (f *fakeIdeaRepo) Create(_ context.Context, i *entity.Idea) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, i)
	return nil
}
func (f *fakeIdeaRepo) GetByID(context.Context, int64) (*entity.Idea, error) { panic("not used") }
func (f *fakeIdeaRepo) GetByProjectID(_ context.Context, userID uuid.UUID, projectID int64, page, limit int) ([]*entity.Idea, int64, error) {
	f.listByProjectArgs = []any{userID, projectID, page, limit}
	return nil, 0, nil
}
func (f *fakeIdeaRepo) Search(context.Context, uuid.UUID, int64, string, []string, int, int) ([]*entity.Idea, int64, error) {
	panic("not used")
}
func (f *fakeIdeaRepo) Update(context.Context, *entity.Idea) error { panic("not used") }
func (f *fakeIdeaRepo) Delete(context.Context, int64) error        { panic("not used") }
func (f *fakeIdeaRepo) ListByTags(context.Context, []string, int64) ([]*entity.Idea, error) {
	panic("not used")
}
func (f *fakeIdeaRepo) SearchBySimilarity(context.Context, int64, []float32, int) ([]*entity.Idea, error) {
	panic("not used")
}
func (f *fakeIdeaRepo) SearchGlobalBySimilarity(context.Context, uuid.UUID, []float32, float64, int) ([]*entity.Idea, error) {
	panic("not used")
}
func (f *fakeIdeaRepo) UpdateEmbedding(context.Context, int64, []float32) error { panic("not used") }
func (f *fakeIdeaRepo) GetWithoutEmbedding(context.Context, int) ([]*entity.Idea, error) {
	panic("not used")
}

func TestIdeaServiceCreateRejectsForeignProject(t *testing.T) {
	userID := uuid.New()
	foreignUser := uuid.New()
	idea := &fakeIdeaRepo{}
	project := &fakeProjectRepo{project: &entity.Project{ID: 7, UserID: foreignUser}}
	store := &repository.DBStore{Project: project, Idea: idea}
	svc := NewIdeaService(store)

	_, err := svc.Create(context.Background(), 7, userID, "content", nil, nil)
	if !errors.Is(err, ErrProjectForbidden) {
		t.Fatalf("expected ErrProjectForbidden, got %v", err)
	}
	if len(idea.created) != 0 {
		t.Fatalf("Idea.Create must not be called for a foreign project, got %d creates", len(idea.created))
	}
}

func TestIdeaServiceCreateOwnProjectSucceeds(t *testing.T) {
	userID := uuid.New()
	idea := &fakeIdeaRepo{}
	project := &fakeProjectRepo{project: &entity.Project{ID: 9, UserID: userID}}
	store := &repository.DBStore{Project: project, Idea: idea}
	svc := NewIdeaService(store)

	created, err := svc.Create(context.Background(), 9, userID, "my idea", map[string]any{"k": "v"}, []string{"tag1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created == nil || created.ID != 0 {
		t.Fatalf("expected created idea, got %#v", created)
	}
	if len(idea.created) != 1 {
		t.Fatalf("expected 1 Idea.Create call, got %d", len(idea.created))
	}
	got := idea.created[0]
	if got.ProjectID != 9 || got.UserID != userID || got.RawInput != "my idea" {
		t.Fatalf("unexpected idea fields: %#v", got)
	}
}

func TestIdeaServiceListByProjectPassesUserID(t *testing.T) {
	userID := uuid.New()
	idea := &fakeIdeaRepo{}
	store := &repository.DBStore{Idea: idea}
	svc := NewIdeaService(store)

	_, _, err := svc.ListByProject(context.Background(), userID, 3, 1, 20)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if len(idea.listByProjectArgs) != 4 {
		t.Fatalf("expected GetByProjectID called, got args %#v", idea.listByProjectArgs)
	}
	if idea.listByProjectArgs[0] != userID || idea.listByProjectArgs[1] != int64(3) {
		t.Fatalf("ListByProject must forward userID and projectID, got %#v", idea.listByProjectArgs)
	}
}
