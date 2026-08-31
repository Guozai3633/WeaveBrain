package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/internal/service"
	"weavebrain/pkg/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// r7FakeDispatcher serves the workflow ownership test through the real HTTP
// handler while reading from the real database row (the handler's ownership
// check compares the returned run's UserID against the authenticated user).
type r7FakeDispatcher struct {
	store *repository.DBStore
}

func (d *r7FakeDispatcher) DispatchIdeaProcess(context.Context, string, string, int64) (*entity.WorkflowRun, error) {
	return nil, errors.New("not used in r7 acceptance test")
}

func (d *r7FakeDispatcher) GetWorkflowStatus(ctx context.Context, workflowID string) (*entity.WorkflowRun, error) {
	return d.store.WorkflowRun.GetByWorkflowID(ctx, workflowID)
}

// TestR7CrossUserSecurityAcceptance proves the five /api/v1 authorization
// fixes plus the timeline 401 bug fix with real PostgreSQL and full HTTP
// requests: one user must never read or write another user's ideas, reminders,
// profile, workflow runs, or hit the timeline regression.
func TestR7CrossUserSecurityAcceptance(t *testing.T) {
	databaseURL := os.Getenv("WEAVEBRAIN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WEAVEBRAIN_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	store := repository.NewFromPool(pool)
	key := []byte("0123456789abcdef0123456789abcdef")
	tokenConfig := auth.TokenConfig{
		Secret: []byte("r7-acceptance-secret"),
		Issuer: "weavebrain",
		Expiry: time.Hour,
	}
	server := NewServer("0", service.New(store, key), tokenConfig, "mock", nil, nil, key)
	server.services.Agent.SetDispatcher(&r7FakeDispatcher{store: store})
	engine := server.engine

	register := func(providerID string) (string, uuid.UUID) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"provider":    "r7_test",
			"provider_id": providerID,
		})
		recorder := performR3Request(engine, http.MethodPost, "/api/v1/auth/register", body, "", "")
		if recorder.Code != http.StatusCreated {
			t.Fatalf("register %s: status=%d body=%s", providerID, recorder.Code, recorder.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode register response: %v", err)
		}
		token, _ := response["token"].(string)
		user, _ := response["user"].(map[string]any)
		userIDText, _ := user["id"].(string)
		userID, err := uuid.Parse(userIDText)
		if err != nil || token == "" || userID == uuid.Nil {
			t.Fatalf("register response incomplete: %s", recorder.Body.String())
		}
		return token, userID
	}

	suffix := uuid.NewString()
	ownerToken, ownerID := register("owner-" + suffix)
	otherToken, _ := register("other-" + suffix)

	// A creates a project.
	projectBody, _ := json.Marshal(map[string]any{"name": "r7-project", "default_project": true})
	projectRec := performR3Request(engine, http.MethodPost, "/api/v1/projects", projectBody, ownerToken, "")
	if projectRec.Code != http.StatusCreated {
		t.Fatalf("create project: status=%d body=%s", projectRec.Code, projectRec.Body.String())
	}
	var projectResp map[string]any
	if err := json.Unmarshal(projectRec.Body.Bytes(), &projectResp); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	project, _ := projectResp["project"].(map[string]any)
	projectID := project["id"].(float64)
	projectIDStr := strconv.FormatFloat(projectID, 'f', -1, 64)

	// A creates an idea in that project.
	ideaBody, _ := json.Marshal(map[string]any{"raw_input": "A's private idea", "tags": []string{"r7"}})
	ideaRec := performR3Request(engine, http.MethodPost, "/api/v1/ideas?project_id="+projectIDStr, ideaBody, ownerToken, "")
	if ideaRec.Code != http.StatusCreated {
		t.Fatalf("create idea: status=%d body=%s", ideaRec.Code, ideaRec.Body.String())
	}

	// A creates a past-due reminder.
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	reminderBody, _ := json.Marshal(map[string]any{"trigger_time": past, "message": "A's reminder"})
	reminderRec := performR3Request(engine, http.MethodPost, "/api/v1/reminders", reminderBody, ownerToken, "")
	if reminderRec.Code != http.StatusCreated {
		t.Fatalf("create reminder: status=%d body=%s", reminderRec.Code, reminderRec.Body.String())
	}

	// Seed A's workflow run for the workflow ownership test.
	workflowID := "r7-wf-" + uuid.NewString()
	if _, err := pool.Exec(
		ctx,
		"INSERT INTO workflow_runs (workflow_id, workflow_type, user_id, status, input, output) VALUES ($1, 'idea_process', $2, 'running', '{}', '{}')",
		workflowID, ownerID,
	); err != nil {
		t.Fatalf("seed workflow run: %v", err)
	}

	// --- B must never see A's data ---
	bIdeas := performR3Request(engine, http.MethodGet, "/api/v1/ideas?project_id="+projectIDStr, nil, otherToken, "")
	if bIdeas.Code != http.StatusOK {
		t.Fatalf("B list A's ideas: status=%d body=%s", bIdeas.Code, bIdeas.Body.String())
	}
	var bIdeasResp map[string]any
	_ = json.Unmarshal(bIdeas.Body.Bytes(), &bIdeasResp)
	if ideas, _ := bIdeasResp["ideas"].([]any); len(ideas) != 0 {
		t.Fatalf("B must see zero of A's ideas, got %d", len(ideas))
	}

	bWriteIdea := performR3Request(
		engine,
		http.MethodPost,
		"/api/v1/ideas?project_id="+projectIDStr,
		[]byte(`{"raw_input":"intrusion"}`),
		otherToken,
		"",
	)
	if bWriteIdea.Code != http.StatusForbidden {
		t.Fatalf("B write to A's project: status=%d body=%s", bWriteIdea.Code, bWriteIdea.Body.String())
	}

	bReminders := performR3Request(engine, http.MethodGet, "/api/v1/reminders", nil, otherToken, "")
	if bReminders.Code != http.StatusOK {
		t.Fatalf("B list reminders: status=%d body=%s", bReminders.Code, bReminders.Body.String())
	}
	var bRemindersResp map[string]any
	_ = json.Unmarshal(bReminders.Body.Bytes(), &bRemindersResp)
	if reminders, _ := bRemindersResp["reminders"].([]any); len(reminders) != 0 {
		t.Fatalf("B must see zero reminders, got %d", len(reminders))
	}

	bUser := performR3Request(engine, http.MethodGet, "/api/v1/users/"+ownerID.String(), nil, otherToken, "")
	if bUser.Code != http.StatusNotFound {
		t.Fatalf("B read A's profile: status=%d body=%s", bUser.Code, bUser.Body.String())
	}

	bWorkflow := performR3Request(engine, http.MethodGet, "/api/v1/agent/workflow/"+workflowID, nil, otherToken, "")
	if bWorkflow.Code != http.StatusForbidden {
		t.Fatalf("B read A's workflow: status=%d body=%s", bWorkflow.Code, bWorkflow.Body.String())
	}

	// --- A's own access must still work ---
	aUser := performR3Request(engine, http.MethodGet, "/api/v1/users/"+ownerID.String(), nil, ownerToken, "")
	if aUser.Code != http.StatusOK {
		t.Fatalf("A read own profile: status=%d body=%s", aUser.Code, aUser.Body.String())
	}

	// BUG-1 regression: users/me/timeline previously 401 forever.
	aTimeline := performR3Request(engine, http.MethodGet, "/api/v1/users/me/timeline", nil, ownerToken, "")
	if aTimeline.Code != http.StatusOK {
		t.Fatalf("A read own timeline: status=%d body=%s", aTimeline.Code, aTimeline.Body.String())
	}

	aWorkflow := performR3Request(engine, http.MethodGet, "/api/v1/agent/workflow/"+workflowID, nil, ownerToken, "")
	if aWorkflow.Code != http.StatusOK {
		t.Fatalf("A read own workflow: status=%d body=%s", aWorkflow.Code, aWorkflow.Body.String())
	}

	aIdeas := performR3Request(engine, http.MethodGet, "/api/v1/ideas?project_id="+projectIDStr, nil, ownerToken, "")
	if aIdeas.Code != http.StatusOK {
		t.Fatalf("A list own ideas: status=%d body=%s", aIdeas.Code, aIdeas.Body.String())
	}
	var aIdeasResp map[string]any
	_ = json.Unmarshal(aIdeas.Body.Bytes(), &aIdeasResp)
	if ideas, _ := aIdeasResp["ideas"].([]any); len(ideas) != 1 {
		t.Fatalf("A must see exactly one idea, got %d", len(ideas))
	}

	aReminders := performR3Request(engine, http.MethodGet, "/api/v1/reminders", nil, ownerToken, "")
	if aReminders.Code != http.StatusOK {
		t.Fatalf("A list own reminders: status=%d body=%s", aReminders.Code, aReminders.Body.String())
	}
	var aRemindersResp map[string]any
	_ = json.Unmarshal(aReminders.Body.Bytes(), &aRemindersResp)
	if reminders, _ := aRemindersResp["reminders"].([]any); len(reminders) != 1 {
		t.Fatalf("A must see exactly one reminder, got %d", len(reminders))
	}
}

