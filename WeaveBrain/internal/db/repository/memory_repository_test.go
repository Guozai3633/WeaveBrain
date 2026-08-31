package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// memoryEntryScan writes a 30-column (capture + memory card) row, matching the
// column order of scanCaptureAndCard used by List and SetLifecycle.
func memoryEntryScan(t *testing.T, capture *entity.Capture, card *entity.MemoryCard) func(...any) error {
	t.Helper()
	return func(dest ...any) error {
		if len(dest) != 30 {
			t.Fatalf("expected 30 scan destinations, got %d", len(dest))
		}
		*(dest[0].(*uuid.UUID)) = capture.ID
		*(dest[1].(*uuid.UUID)) = capture.UserID
		*(dest[2].(*string)) = string(capture.Kind)
		*(dest[3].(**string)) = capture.OriginalText
		*(dest[4].(**time.Time)) = capture.CapturedAt
		*(dest[5].(*string)) = capture.CapturedAtPrecision
		*(dest[6].(**string)) = capture.Timezone
		*(dest[7].(*string)) = capture.Source
		*(dest[8].(**int64)) = capture.CollectionID
		*(dest[9].(*string)) = capture.PrivacyMode
		*(dest[10].(*string)) = capture.RequestHash
		*(dest[11].(*int)) = capture.ClientVersion
		*(dest[12].(*int64)) = capture.Version
		*(dest[13].(*string)) = capture.LifecycleStatus
		*(dest[14].(*time.Time)) = capture.CreatedAt
		*(dest[15].(*time.Time)) = capture.UpdatedAt
		*(dest[16].(*uuid.UUID)) = card.ID
		*(dest[17].(*uuid.UUID)) = card.UserID
		*(dest[18].(*uuid.UUID)) = card.CaptureID
		*(dest[19].(*string)) = card.PrimaryType
		*(dest[20].(*string)) = card.Title
		*(dest[21].(**string)) = card.Summary
		tagsJSON, _ := json.Marshal(card.Tags)
		*(dest[22].(*[]byte)) = tagsJSON
		keyPointsJSON, _ := json.Marshal(card.KeyPoints)
		*(dest[23].(*[]byte)) = keyPointsJSON
		*(dest[24].(*string)) = card.ProcessingStatus
		*(dest[25].(*int64)) = card.Version
		*(dest[26].(*bool)) = card.IsPinned
		*(dest[27].(**time.Time)) = card.PinnedAt
		*(dest[28].(*time.Time)) = card.CreatedAt
		*(dest[29].(*time.Time)) = card.UpdatedAt
		return nil
	}
}

// memoryRevisionScan writes a 10-column enrichment revision row, matching
// scanEnrichmentRevision used by ListRevisions.
func memoryRevisionScan(t *testing.T, rev *entity.EnrichmentRevision) func(...any) error {
	t.Helper()
	return func(dest ...any) error {
		if len(dest) != 10 {
			t.Fatalf("expected 10 scan destinations, got %d", len(dest))
		}
		*(dest[0].(*int64)) = rev.ID
		*(dest[1].(*uuid.UUID)) = rev.UserID
		*(dest[2].(*uuid.UUID)) = rev.CaptureID
		*(dest[3].(*int32)) = rev.Revision
		*(dest[4].(*int64)) = rev.CardVersion
		*(dest[5].(*string)) = string(rev.Source)
		*(dest[6].(*int64)) = rev.SourceRevision
		changesJSON, _ := json.Marshal(rev.Changes)
		*(dest[7].(*[]byte)) = changesJSON
		provJSON, _ := json.Marshal(rev.Provenance)
		*(dest[8].(*[]byte)) = provJSON
		*(dest[9].(*time.Time)) = rev.CreatedAt
		return nil
	}
}

// memoryCardScan writes a 14-column memory card row, matching
// scanMemoryCardRow used by SetPinned.
func memoryCardScan(t *testing.T, card *entity.MemoryCard) func(...any) error {
	t.Helper()
	return func(dest ...any) error {
		if len(dest) != 14 {
			t.Fatalf("expected 14 scan destinations, got %d", len(dest))
		}
		*(dest[0].(*uuid.UUID)) = card.ID
		*(dest[1].(*uuid.UUID)) = card.UserID
		*(dest[2].(*uuid.UUID)) = card.CaptureID
		*(dest[3].(*string)) = card.PrimaryType
		*(dest[4].(*string)) = card.Title
		*(dest[5].(**string)) = card.Summary
		tagsJSON, _ := json.Marshal(card.Tags)
		*(dest[6].(*[]byte)) = tagsJSON
		keyPointsJSON, _ := json.Marshal(card.KeyPoints)
		*(dest[7].(*[]byte)) = keyPointsJSON
		*(dest[8].(*string)) = card.ProcessingStatus
		*(dest[9].(*int64)) = card.Version
		*(dest[10].(*bool)) = card.IsPinned
		*(dest[11].(**time.Time)) = card.PinnedAt
		*(dest[12].(*time.Time)) = card.CreatedAt
		*(dest[13].(*time.Time)) = card.UpdatedAt
		return nil
	}
}

// memoryRevisionAndCardScan writes the 24-column row returned by
// AppendRevisionAndUpdateCard / AppendNoteAndBump (scanRevisionAndCard).
func memoryRevisionAndCardScan(t *testing.T, rev *entity.EnrichmentRevision, card *entity.MemoryCard) func(...any) error {
	t.Helper()
	return func(dest ...any) error {
		if len(dest) != 24 {
			t.Fatalf("expected 24 scan destinations, got %d", len(dest))
		}
		revScan := memoryRevisionScan(t, rev)
		cardScan := memoryCardScan(t, card)
		if err := revScan(dest[:10]...); err != nil {
			return err
		}
		if err := cardScan(dest[10:]...); err != nil {
			return err
		}
		return nil
	}
}

func memoryTestCapture(userID, captureID uuid.UUID, createdAt time.Time) *entity.Capture {
	return &entity.Capture{
		ID:                  captureID,
		UserID:              userID,
		Kind:                entity.CaptureKindText,
		OriginalText:        strPtr("original memory text"),
		CapturedAtPrecision: "minute",
		Source:              "web",
		PrivacyMode:         "cloud_allowed",
		ClientVersion:       1,
		Version:             1,
		LifecycleStatus:     "active",
		CreatedAt:           createdAt,
		UpdatedAt:           createdAt,
	}
}

func memoryTestCard(userID, captureID uuid.UUID, createdAt time.Time) *entity.MemoryCard {
	return &entity.MemoryCard{
		ID:               uuid.New(),
		UserID:           userID,
		CaptureID:        captureID,
		PrimaryType:      "uncategorized",
		Title:            "original memory text",
		Summary:          strPtr("original memory text"),
		Tags:             []string{},
		KeyPoints:        []string{},
		ProcessingStatus: "ready",
		Version:          1,
		IsPinned:         false,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
}

func strPtr(s string) *string { return &s }

func strPtrTime(t time.Time) *time.Time { return &t }

func TestMemoryRepositoryListScopesOwnershipAndDefaults(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	now := time.Now().UTC()
	entry := &entity.MemoryListEntry{
		Capture:    memoryTestCapture(userID, captureID, now),
		MemoryCard: memoryTestCard(userID, captureID, now),
	}
	conn := &outboxTestConn{
		rows: &outboxTestRows{scans: []func(...any) error{
			memoryEntryScan(t, entry.Capture, entry.MemoryCard),
		}},
	}
	repo := NewMemoryRepository(conn)

	result, err := repo.List(context.Background(), userID, entity.MemoryListQuery{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result.Items))
	}
	if result.Items[0].Capture.ID != captureID {
		t.Fatalf("unexpected capture id %v", result.Items[0].Capture.ID)
	}
	if result.Items[0].MemoryCard.Title != "original memory text" {
		t.Fatalf("unexpected card title %q", result.Items[0].MemoryCard.Title)
	}
	if len(conn.args) != 10 {
		t.Fatalf("expected 10 bound args, got %d", len(conn.args))
	}
	if conn.args[0] != userID {
		t.Fatalf("expected user id first arg, got %#v", conn.args[0])
	}
	if conn.args[1] != "active" {
		t.Fatalf("expected default lifecycle 'active', got %#v", conn.args[1])
	}
	if conn.args[9] != 21 {
		t.Fatalf("expected limit+1 = 21, got %#v", conn.args[9])
	}

	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"JOIN memory_cards m",
		"WHERE c.user_id = $1",
		"c.deleted_at IS NULL",
		"c.lifecycle_status = $2",
		"ORDER BY m.is_pinned DESC, c.created_at DESC, c.id DESC",
		"LIMIT $10",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("List query missing %q:\n%s", check, normalized)
		}
	}
}

func TestMemoryRepositoryListPassesFiltersAndCursor(t *testing.T) {
	userID := uuid.New()
	now := time.Now().UTC()
	cursorToken := encodeMemoryCursor(true, now, uuid.New())

	conn := &outboxTestConn{rows: &outboxTestRows{}}
	repo := NewMemoryRepository(conn)

	_, err := repo.List(context.Background(), userID, entity.MemoryListQuery{
		Cursor:       &cursorToken,
		Limit:        30,
		Q:            "你好%世界_",
		Kind:         "text",
		PrimaryType:  "idea",
		PinnedOnly:   true,
		LifecycleStatus: "archived",
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(conn.args) != 10 {
		t.Fatalf("expected 10 bound args, got %d", len(conn.args))
	}
	if conn.args[0] != userID || conn.args[1] != "archived" {
		t.Fatalf("unexpected user/lifecycle args: %#v", conn.args[:2])
	}
	if conn.args[2] != "text" {
		t.Fatalf("expected kind filter 'text', got %#v", conn.args[2])
	}
	if conn.args[3] != "idea" {
		t.Fatalf("expected primary_type filter 'idea', got %#v", conn.args[3])
	}
	if conn.args[4] != true {
		t.Fatalf("expected pinned_only true, got %#v", conn.args[4])
	}
	search, ok := conn.args[5].(string)
	if !ok {
		t.Fatalf("expected search pattern string, got %T", conn.args[5])
	}
	if search != `%你好\%世界\_%` {
		t.Fatalf("expected escaped LIKE pattern, got %q", search)
	}
	if conn.args[6] != true {
		t.Fatalf("expected cursor pinned true, got %#v", conn.args[6])
	}
	if conn.args[9] != 31 {
		t.Fatalf("expected limit+1 = 31, got %#v", conn.args[9])
	}

	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"($3::text IS NULL OR c.kind = $3)",
		"($4::text IS NULL OR m.primary_type = $4)",
		"($5::boolean = FALSE OR m.is_pinned = TRUE)",
		"m.title ILIKE $6 ESCAPE '\\'",
		"c.original_text ILIKE $6 ESCAPE '\\'",
		"FROM transcript_revisions tr",
		"($7::boolean IS NULL OR (m.is_pinned, c.created_at, c.id) < ($7, $8, $9))",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("List filter query missing %q:\n%s", check, normalized)
		}
	}
}

func TestMemoryRepositoryListComputesNextCursor(t *testing.T) {
	userID := uuid.New()
	base := time.Now().UTC()
	created1 := base.Add(-3 * time.Hour)
	created2 := base.Add(-2 * time.Hour)
	created3 := base.Add(-1 * time.Hour)
	c1, c2, c3 := uuid.New(), uuid.New(), uuid.New()
	entries := []*entity.MemoryListEntry{
		{Capture: memoryTestCapture(userID, c1, created1), MemoryCard: memoryTestCard(userID, c1, created1)},
		{Capture: memoryTestCapture(userID, c2, created2), MemoryCard: memoryTestCard(userID, c2, created2)},
		{Capture: memoryTestCapture(userID, c3, created3), MemoryCard: memoryTestCard(userID, c3, created3)},
	}
	scans := make([]func(...any) error, 0, 3)
	for _, e := range entries {
		scans = append(scans, memoryEntryScan(t, e.Capture, e.MemoryCard))
	}
	conn := &outboxTestConn{rows: &outboxTestRows{scans: scans}}
	repo := NewMemoryRepository(conn)

	result, err := repo.List(context.Background(), userID, entity.MemoryListQuery{Limit: 2})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items with limit 2, got %d", len(result.Items))
	}
	if result.NextCursor == nil {
		t.Fatal("expected next_cursor when more rows exist")
	}
	// The cursor must resume from the last returned item (the 2nd row).
	want := encodeMemoryCursor(entries[1].MemoryCard.IsPinned, entries[1].Capture.CreatedAt, entries[1].Capture.ID)
	if *result.NextCursor != want {
		t.Fatalf("cursor = %q, want %q", *result.NextCursor, want)
	}
	if len(conn.args) != 10 || conn.args[9] != 3 {
		t.Fatalf("expected limit+1 = 3 fetched rows, got %#v", conn.args)
	}
}

func TestMemoryRepositoryListRejectsInvalidCursor(t *testing.T) {
	bad := "***not-base64***"
	conn := &outboxTestConn{}
	repo := NewMemoryRepository(conn)

	_, err := repo.List(context.Background(), uuid.New(), entity.MemoryListQuery{Cursor: &bad})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestMemoryRepositoryListRevisionsNewestFirst(t *testing.T) {
	userID, captureID := uuid.New(), uuid.New()
	rev1 := &entity.EnrichmentRevision{
		ID: 1, UserID: userID, CaptureID: captureID, Revision: 1, CardVersion: 1,
		Source: entity.EnrichmentSourceFallback, SourceRevision: 1,
		Changes: map[string]any{"title": "first"}, Provenance: map[string]any{"source": "fallback"},
		CreatedAt: time.Now().UTC(),
	}
	rev2 := &entity.EnrichmentRevision{
		ID: 2, UserID: userID, CaptureID: captureID, Revision: 2, CardVersion: 2,
		Source: entity.EnrichmentSourceUser, SourceRevision: 1,
		Changes: map[string]any{"title": "second"}, Provenance: map[string]any{"title": "user"},
		CreatedAt: time.Now().UTC(),
	}
	conn := &outboxTestConn{
		rows: &outboxTestRows{scans: []func(...any) error{
			memoryRevisionScan(t, rev2),
			memoryRevisionScan(t, rev1),
		}},
	}
	repo := NewMemoryRepository(conn)

	got, err := repo.ListRevisions(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("ListRevisions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(got))
	}
	if got[0].Revision != 2 || got[1].Revision != 1 {
		t.Fatalf("expected newest-first ordering, got revisions %d,%d", got[0].Revision, got[1].Revision)
	}
	if got[0].Source != entity.EnrichmentSourceUser {
		t.Fatalf("expected user source, got %q", got[0].Source)
	}
	if len(conn.args) != 2 || conn.args[0] != userID || conn.args[1] != captureID {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "ORDER BY revision DESC") {
		t.Fatalf("expected revision DESC ordering:\n%s", normalized)
	}
}

func TestMemoryRepositoryAppendRevisionAndUpdateCardAtomicCTE(t *testing.T) {
	userID, captureID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	rev := &entity.EnrichmentRevision{
		UserID: userID, CaptureID: captureID,
		Source: entity.EnrichmentSourceUser, SourceRevision: 1,
		Changes: map[string]any{"title": "新标题"}, Provenance: map[string]any{"title": "user"},
	}
	card := &entity.MemoryCard{
		ID: uuid.New(), UserID: userID, CaptureID: captureID,
		PrimaryType: "idea", Title: "新标题", Summary: strPtr("summary"),
		Tags: []string{"a", "b"}, KeyPoints: []string{"kp"},
		ProcessingStatus: "ready", Version: 2, IsPinned: false,
		CreatedAt: now, UpdatedAt: now,
	}
	resultRev := *rev
	resultRev.ID = 7
	resultRev.Revision = 2
	resultRev.CardVersion = 2
	resultRev.CreatedAt = now
	conn := &outboxTestConn{
		row: outboxTestRow{scan: memoryRevisionAndCardScan(t, &resultRev, card)},
	}
	repo := NewMemoryRepository(conn)

	gotRev, gotCard, err := repo.AppendRevisionAndUpdateCard(context.Background(), userID, captureID, rev, card)
	if err != nil {
		t.Fatalf("AppendRevisionAndUpdateCard: %v", err)
	}
	if gotRev.Revision != 2 || gotRev.CardVersion != 2 {
		t.Fatalf("expected revision 2 / card version 2, got %d/%d", gotRev.Revision, gotRev.CardVersion)
	}
	if gotCard.Title != "新标题" || gotCard.PrimaryType != "idea" {
		t.Fatalf("unexpected refreshed card: %#v", gotCard)
	}
	if len(conn.args) != 11 {
		t.Fatalf("expected 11 bound args, got %d", len(conn.args))
	}
	if conn.args[0] != userID || conn.args[1] != captureID {
		t.Fatalf("unexpected ownership args: %#v", conn.args[:2])
	}
	if conn.args[2] != "user" {
		t.Fatalf("expected source 'user', got %#v", conn.args[2])
	}
	if conn.args[3] != int64(1) {
		t.Fatalf("expected source_revision 1, got %#v", conn.args[3])
	}
	changesJSON, ok := conn.args[4].([]byte)
	if !ok || !strings.Contains(string(changesJSON), "新标题") {
		t.Fatalf("expected changes JSON bound, got %#v", conn.args[4])
	}
	if conn.args[6] != "新标题" {
		t.Fatalf("expected card title bound, got %#v", conn.args[6])
	}
	tags, ok := conn.args[9].([]string)
	if !ok || len(tags) != 2 {
		t.Fatalf("expected tags bound as []string, got %T %#v", conn.args[9], conn.args[9])
	}
	keyPoints, ok := conn.args[10].([]string)
	if !ok || len(keyPoints) != 1 {
		t.Fatalf("expected key_points bound as []string, got %T %#v", conn.args[10], conn.args[10])
	}

	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"WITH inserted_revision AS",
		"INSERT INTO memory_card_revisions",
		"COALESCE( (SELECT MAX(rev.revision)",
		"mc.version + 1",
		"FROM memory_cards mc",
		"UPDATE memory_cards mc",
		"tags = $10::jsonb",
		"key_points = $11::jsonb",
		"RETURNING",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("AppendRevisionAndUpdateCard query missing %q:\n%s", check, normalized)
		}
	}
}

func TestMemoryRepositoryAppendNoteAndBumpOnlyBumpsVersion(t *testing.T) {
	userID, captureID := uuid.New(), uuid.New()
	rev := &entity.EnrichmentRevision{
		UserID: userID, CaptureID: captureID,
		Source: entity.EnrichmentSourceUser, SourceRevision: 1,
		Changes: map[string]any{"note": "继续想"}, Provenance: map[string]any{"note": "user"},
	}
	card := &entity.MemoryCard{
		ID: uuid.New(), UserID: userID, CaptureID: captureID,
		PrimaryType: "uncategorized", Title: "原文标题", Tags: []string{}, KeyPoints: []string{},
		ProcessingStatus: "ready", Version: 2,
	}
	resultRev := *rev
	resultRev.ID = 8
	resultRev.Revision = 2
	resultRev.CardVersion = 2
	conn := &outboxTestConn{
		row: outboxTestRow{scan: memoryRevisionAndCardScan(t, &resultRev, card)},
	}
	repo := NewMemoryRepository(conn)

	_, gotCard, err := repo.AppendNoteAndBump(context.Background(), userID, captureID, rev)
	if err != nil {
		t.Fatalf("AppendNoteAndBump: %v", err)
	}
	if gotCard.Title != "原文标题" {
		t.Fatalf("note must not rewrite card title, got %q", gotCard.Title)
	}
	if len(conn.args) != 6 {
		t.Fatalf("expected 6 bound args, got %d", len(conn.args))
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"INSERT INTO memory_card_revisions",
		"SET version = mc.version + 1",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("AppendNoteAndBump query missing %q:\n%s", check, normalized)
		}
	}
}

func TestMemoryRepositorySetPinnedTogglesPinnedAt(t *testing.T) {
	userID, captureID := uuid.New(), uuid.New()
	card := memoryTestCard(userID, captureID, time.Now().UTC())
	card.IsPinned = true
	card.PinnedAt = strPtrTime(time.Now().UTC())
	conn := &outboxTestConn{
		row: outboxTestRow{scan: memoryCardScan(t, card)},
	}
	repo := NewMemoryRepository(conn)

	got, err := repo.SetPinned(context.Background(), userID, captureID, true)
	if err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	if !got.IsPinned {
		t.Fatal("expected card pinned")
	}
	if len(conn.args) != 3 || conn.args[0] != userID || conn.args[1] != captureID || conn.args[2] != true {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "is_pinned = $3") ||
		!strings.Contains(normalized, "pinned_at = CASE WHEN $3 THEN now() ELSE NULL END") {
		t.Fatalf("SetPinned query incorrect:\n%s", normalized)
	}
}

func TestMemoryRepositorySetLifecycleTrashedSetsDeletedAt(t *testing.T) {
	userID, captureID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	capture := memoryTestCapture(userID, captureID, now)
	capture.LifecycleStatus = "trashed"
	card := memoryTestCard(userID, captureID, now)
	// deleted_at is not one of the scanned columns (scanCaptureAndCard), so we
	// only verify the query sets it for trashed/deleted below.
	conn := &outboxTestConn{
		row: outboxTestRow{scan: memoryEntryScan(t, capture, card)},
	}
	repo := NewMemoryRepository(conn)

	agg, err := repo.SetLifecycle(context.Background(), userID, captureID, "trashed")
	if err != nil {
		t.Fatalf("SetLifecycle: %v", err)
	}
	if agg.Capture.LifecycleStatus != "trashed" {
		t.Fatalf("expected trashed lifecycle, got %q", agg.Capture.LifecycleStatus)
	}
	if len(conn.args) != 3 || conn.args[0] != userID || conn.args[1] != captureID || conn.args[2] != "trashed" {
		t.Fatalf("unexpected args: %#v", conn.args)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	checks := []string{
		"UPDATE captures c",
		"SET lifecycle_status = $3::text",
		"deleted_at = CASE WHEN $3::text IN ('trashed', 'deleted') THEN now() ELSE NULL END",
		"WHERE c.user_id = $1 AND c.id = $2 AND c.deleted_at IS NULL",
		"JOIN memory_cards m",
	}
	for _, check := range checks {
		if !strings.Contains(normalized, check) {
			t.Fatalf("SetLifecycle query missing %q:\n%s", check, normalized)
		}
	}
}

func TestMemoryRepositorySetLifecycleArchiveKeepsDeletedAtNull(t *testing.T) {
	userID, captureID := uuid.New(), uuid.New()
	now := time.Now().UTC()
	capture := memoryTestCapture(userID, captureID, now)
	capture.LifecycleStatus = "archived"
	card := memoryTestCard(userID, captureID, now)
	conn := &outboxTestConn{
		row: outboxTestRow{scan: memoryEntryScan(t, capture, card)},
	}
	repo := NewMemoryRepository(conn)

	agg, err := repo.SetLifecycle(context.Background(), userID, captureID, "archived")
	if err != nil {
		t.Fatalf("SetLifecycle: %v", err)
	}
	if agg.Capture.LifecycleStatus != "archived" {
		t.Fatalf("expected archived lifecycle, got %q", agg.Capture.LifecycleStatus)
	}
	normalized := strings.Join(strings.Fields(conn.query), " ")
	if !strings.Contains(normalized, "deleted_at = CASE WHEN $3::text IN ('trashed', 'deleted') THEN now() ELSE NULL END") {
		t.Fatalf("archive must keep deleted_at NULL:\n%s", normalized)
	}
}
