package service

import (
	"strings"
	"testing"
)

func TestExtractCompletionJSONArray(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{"clean array", `[{"field":"tags"}]`, `[{"field":"tags"}]`, false},
		{"code fence", "```json\n[{\"field\":\"tags\"}]\n```", `[{"field":"tags"}]`, false},
		{"surrounding prose", "好的，以下是结果：\n[{\"field\":\"tags\"}]\n希望对你有帮助", `[{"field":"tags"}]`, false},
		{"empty", "", "", true},
		{"no brackets", "just text", "", true},
		{"empty array", `[]`, `[]`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := extractCompletionJSONArray(c.content)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestValidateCompletionFieldValue(t *testing.T) {
	goodTags := []any{"工作", "灵感"}
	tooManyTags := make([]any, maxMemoryTags+1)
	for i := range tooManyTags {
		tooManyTags[i] = "x"
	}
	goodKeyPoints := []any{"即时保存"}
	tooLongKeyPoint := []any{strings.Repeat("长", maxMemoryKeyPointRunes+1)}

	cases := []struct {
		name  string
		field string
		raw   any
		ok    bool
	}{
		{"primary type valid", "primary_type", "idea", true},
		{"primary type invalid", "primary_type", "bogus", false},
		{"primary type must be string", "primary_type", []any{"idea"}, false},
		{"tags array", "tags", goodTags, true},
		{"tags too many", "tags", tooManyTags, false},
		{"tags must be array", "tags", "工作", false},
		{"tags empty array", "tags", []any{}, false},
		{"tags non-string item", "tags", []any{1}, false},
		{"key_points array", "key_points", goodKeyPoints, true},
		{"key_points too long item", "key_points", tooLongKeyPoint, false},
		{"title string", "title", "标题", true},
		{"title empty", "title", "", false},
		{"title too long", "title", strings.Repeat("长", maxMemoryTitleRunes+1), false},
		{"summary string", "summary", "摘要", true},
		{"summary too long", "summary", strings.Repeat("长", maxMemorySummaryRunes+1), false},
		{"unknown field", "location", "x", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ok := validateCompletionFieldValue(c.field, c.raw)
			if ok != c.ok {
				t.Fatalf("validateCompletionFieldValue(%s, %#v) ok=%v, want %v", c.field, c.raw, ok, c.ok)
			}
		})
	}
}

func TestClampConfidence(t *testing.T) {
	high := 2.0
	got := clampConfidence(&high)
	if got == nil || *got != 1.0 {
		t.Fatalf("expected clamped to 1.0, got %v", got)
	}
	neg := -1.0
	got = clampConfidence(&neg)
	if got == nil || *got != 0.0 {
		t.Fatalf("expected clamped to 0.0, got %v", got)
	}
	if clampConfidence(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}
