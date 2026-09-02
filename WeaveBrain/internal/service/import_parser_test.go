package service

import (
	"context"
	"strings"
	"testing"

	"weavebrain/internal/entity"
)

func rowHasError(row ParsedImportRow, code string) bool {
	for _, e := range row.Errors {
		if e.Code == code {
			return true
		}
	}
	return false
}

func TestPlainTextParserSplitsBlocks(t *testing.T) {
	raw := `第一条想法

这是第一条的第二段。

---

第二条想法
---
第三条想法`
	parser := newImportParser(entity.ImportFormatPlainText, "")
	rows, mapping, err := parser.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows)=%d, want 3", len(rows))
	}
	if rows[0].RowNumber != 1 || rows[1].RowNumber != 2 || rows[2].RowNumber != 3 {
		t.Fatalf("row numbers = %d/%d/%d, want 1/2/3", rows[0].RowNumber, rows[1].RowNumber, rows[2].RowNumber)
	}
	// Internal blank line inside block 1 must be preserved as a paragraph.
	if !strings.Contains(rows[0].Content, "第一条的第二段。") || !strings.Contains(rows[0].Content, "\n") {
		t.Fatalf("block 1 content lost internal paragraph: %q", rows[0].Content)
	}
	if rows[1].Content != "第二条想法" {
		t.Fatalf("block 2 content = %q", rows[1].Content)
	}
	if rows[2].Content != "第三条想法" {
		t.Fatalf("block 3 content = %q", rows[2].Content)
	}
	if mapping["content"] != "content" {
		t.Fatalf("unexpected column mapping: %v", mapping)
	}
}

func TestPlainTextParserCustomSeparatorAndEmptyBlocks(t *testing.T) {
	raw := "A\n\n===\n\n\n===\nB"
	parser := newImportParser(entity.ImportFormatPlainText, "===")
	rows, _, err := parser.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows)=%d, want 2 (empty blocks dropped)", len(rows))
	}
	if rows[0].Content != "A" || rows[1].Content != "B" {
		t.Fatalf("unexpected contents: %q, %q", rows[0].Content, rows[1].Content)
	}
}

func TestCSVParserFieldMapping(t *testing.T) {
	raw := "External_ID,正文,标题,时间,标签,lat,lng\n" +
		"t1,第一条,标题一,2026-01-02 10:30:00,工作|灵感,31.23,121.47\n" +
		"t2,第二条,,2026-01-03,,30.0,120.0\n"
	parser := newImportParser(entity.ImportFormatCSV, "")
	rows, mapping, err := parser.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows)=%d, want 2", len(rows))
	}

	row := rows[0]
	if row.ExternalID == nil || *row.ExternalID != "t1" {
		t.Fatalf("external_id = %v, want t1", row.ExternalID)
	}
	if row.Content != "第一条" {
		t.Fatalf("content = %q", row.Content)
	}
	if row.Title == nil || *row.Title != "标题一" {
		t.Fatalf("title = %v", row.Title)
	}
	if row.CapturedAt == nil || row.CapturedAtPrecision != "exact" {
		t.Fatalf("captured_at precision = %q, want exact", row.CapturedAtPrecision)
	}
	if len(row.Tags) != 2 || row.Tags[0] != "工作" || row.Tags[1] != "灵感" {
		t.Fatalf("tags = %v, want [工作 灵感]", row.Tags)
	}
	if row.Latitude == nil || *row.Latitude != 31.23 {
		t.Fatalf("latitude = %v, want 31.23", row.Latitude)
	}
	if row.Longitude == nil || *row.Longitude != 121.47 {
		t.Fatalf("longitude = %v, want 121.47", row.Longitude)
	}
	if len(row.Errors) != 0 {
		t.Fatalf("row 1 errors: %v", row.Errors)
	}

	row2 := rows[1]
	if len(row2.Tags) != 0 {
		t.Fatalf("row 2 tags = %v, want empty", row2.Tags)
	}
	if row2.Title != nil {
		t.Fatalf("row 2 title = %v, want nil", row2.Title)
	}

	// Column mapping should map canonical field -> original header.
	for field, header := range map[string]string{
		"external_id": "External_ID",
		"content":     "正文",
		"title":       "标题",
		"captured_at": "时间",
		"tags":        "标签",
		"latitude":    "lat",
		"longitude":   "lng",
	} {
		if mapping[field] != header {
			t.Fatalf("mapping[%q] = %v, want %q", field, mapping[field], header)
		}
	}
}

func TestCSVParserDateOnlyAndInvalidCoordinates(t *testing.T) {
	raw := "content,captured_at,latitude,longitude\n" +
		"第一行,2026-05-01,10,20\n" +          // bare date -> date_only, valid coords
		"第二行,,10,\n" +                       // empty date ok, lone lat -> invalid_coordinates
		"第三行,2026-13-99,200,-500\n" +        // bad date + out-of-range coords
		",2026-01-01,,\n"                       // empty content -> missing_content
	parser := newImportParser(entity.ImportFormatCSV, "")
	rows, _, err := parser.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("len(rows)=%d, want 4 (bad rows isolated, not fatal)", len(rows))
	}

	r0 := rows[0]
	if r0.CapturedAtPrecision != "date_only" {
		t.Fatalf("row 1 precision = %q, want date_only", r0.CapturedAtPrecision)
	}
	if len(r0.Errors) != 0 {
		t.Fatalf("row 1 errors: %v", r0.Errors)
	}

	r1 := rows[1]
	if !rowHasError(r1, "invalid_coordinates") {
		t.Fatalf("row 2 should flag invalid_coordinates: %v", r1.Errors)
	}
	if r1.CapturedAtPrecision != "unknown" {
		t.Fatalf("row 2 empty captured_at should stay unknown, got %q", r1.CapturedAtPrecision)
	}

	r2 := rows[2]
	if !rowHasError(r2, "invalid_date") {
		t.Fatalf("row 3 should flag invalid_date: %v", r2.Errors)
	}
	if !rowHasError(r2, "invalid_coordinates") {
		t.Fatalf("row 3 should flag invalid_coordinates: %v", r2.Errors)
	}

	r3 := rows[3]
	if !rowHasError(r3, "missing_content") {
		t.Fatalf("row 4 should flag missing_content: %v", r3.Errors)
	}
}

func TestJSONLParserValidBadMissingContent(t *testing.T) {
	raw := `{"external_id":"j1","content":"合法内容","tags":["工作","灵感"],"captured_at":"2026-01-02T10:30:00Z"}
{"external_id":"j2","content":"第二条","latitude":31.2,"longitude":121.4}
这不是 JSON
{"external_id":"j4"}
`
	parser := newImportParser(entity.ImportFormatJSONL, "")
	rows, mapping, err := parser.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("len(rows)=%d, want 4", len(rows))
	}

	r0 := rows[0]
	if r0.ExternalID == nil || *r0.ExternalID != "j1" {
		t.Fatalf("row 1 external_id = %v", r0.ExternalID)
	}
	if len(r0.Tags) != 2 || r0.Tags[0] != "工作" {
		t.Fatalf("row 1 tags = %v", r0.Tags)
	}
	if r0.CapturedAtPrecision != "exact" {
		t.Fatalf("row 1 precision = %q, want exact", r0.CapturedAtPrecision)
	}
	if len(r0.Errors) != 0 {
		t.Fatalf("row 1 errors: %v", r0.Errors)
	}

	r1 := rows[1]
	if r1.Latitude == nil || *r1.Latitude != 31.2 {
		t.Fatalf("row 2 latitude = %v", r1.Latitude)
	}

	r2 := rows[2]
	if !rowHasError(r2, "invalid_json") {
		t.Fatalf("row 3 should flag invalid_json: %v", r2.Errors)
	}

	r3 := rows[3]
	if !rowHasError(r3, "missing_content") {
		t.Fatalf("row 4 should flag missing_content: %v", r3.Errors)
	}

	if mapping["external_id"] != "external_id" || mapping["content"] != "content" {
		t.Fatalf("unexpected jsonl column mapping: %v", mapping)
	}
}

func TestParsedCapturedAtFormats(t *testing.T) {
	cases := []struct {
		in        string
		precision string
	}{
		{"2026-01-02T10:30:00Z", "exact"},
		{"2026-01-02T10:30:00+08:00", "exact"},
		{"2026-01-02 10:30:00", "exact"},
		{"2026-01-02 10:30", "exact"},
		{"2026-01-02", "date_only"},
		{"", "unknown"},
	}
	for _, tc := range cases {
		parsed, precision, err := parseCapturedAt(tc.in)
		if err != nil {
			t.Fatalf("parseCapturedAt(%q): %v", tc.in, err)
		}
		if precision != tc.precision {
			t.Fatalf("parseCapturedAt(%q) precision=%q, want %q", tc.in, precision, tc.precision)
		}
		if tc.in != "" && parsed == nil {
			t.Fatalf("parseCapturedAt(%q) parsed nil", tc.in)
		}
	}
	if _, _, err := parseCapturedAt("not-a-date"); err == nil {
		t.Fatal("expected error for invalid date")
	}
}

func TestParseTagsValue(t *testing.T) {
	if got := parseTagsValue("a | b||c "); len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("string tags = %v", got)
	}
	if got := parseTagsValue([]any{"x", "y"}); len(got) != 2 || got[0] != "x" {
		t.Fatalf("array tags = %v", got)
	}
	if got := parseTagsValue(42); got != nil {
		t.Fatalf("scalar tags = %v, want nil", got)
	}
}

