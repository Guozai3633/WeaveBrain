package service

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"weavebrain/internal/entity"
)

// DefaultImportSeparator is the line that separates blocks in a plain-text /
// TXT / Markdown batch import.
const DefaultImportSeparator = "---"

// maxImportRawBytes is the raw text budget for one batch import request.
// The HTTP layer enforces it with MaxBytesReader; the parsers also size their
// streaming buffers to it so a single giant block never trips a scanner limit.
const maxImportRawBytes = 8 << 20 // 8 MiB

// maxImportRows is the MVP row budget for a single import job. Imports that
// exceed it are rejected up front rather than committed in unbounded batches.
const maxImportRows = 1000

// ParsedImportRow is one row parsed from raw import text. Location fields are
// validated for the report but intentionally NOT persisted into captures (the
// captures table has no columns for them); they live in RawPayload so a future
// release can promote them.
type ParsedImportRow struct {
	RowNumber           int
	ExternalID          *string
	Content             string
	Title               *string
	CapturedAt          *time.Time
	CapturedAtPrecision string
	Timezone            *string
	Tags                []string
	SourceURL           *string
	LocationName        *string
	Latitude            *float64
	Longitude           *float64
	Activity            *string
	RawPayload          map[string]any
	Errors              []entity.ImportValidationError
}

// ImportParser turns raw import text into a slice of rows plus the derived
// column mapping stored on the job. Parsers are streaming (bufio.Scanner) and
// row-level error tolerant: a bad row is reported in its Errors, never aborting
// the whole parse (G7 criterion 1).
type ImportParser interface {
	Parse(ctx context.Context, rawText string) ([]ParsedImportRow, map[string]any, error)
}

// newImportParser returns the parser for a format. TXT/Markdown are aliases of
// plain_text handled at the handler layer before reaching the service.
func newImportParser(format entity.ImportFormat, separator string) ImportParser {
	switch format {
	case entity.ImportFormatCSV:
		return &csvImportParser{}
	case entity.ImportFormatJSONL:
		return &jsonlImportParser{}
	default:
		return &plainTextImportParser{separator: separator}
	}
}

// ---------------------------------------------------------------------------
// Plain text / TXT / Markdown
// ---------------------------------------------------------------------------

type plainTextImportParser struct {
	separator string
}

func (p *plainTextImportParser) Parse(_ context.Context, rawText string) ([]ParsedImportRow, map[string]any, error) {
	separator := p.separator
	if separator == "" {
		separator = DefaultImportSeparator
	}
	var rows []ParsedImportRow
	for _, block := range splitBySeparator(rawText, separator) {
		content := strings.TrimSpace(block)
		if content == "" {
			continue
		}
		rows = append(rows, ParsedImportRow{
			RowNumber:  len(rows) + 1,
			Content:    content,
			RawPayload: map[string]any{"content": content},
		})
	}
	return rows, map[string]any{"content": "content"}, nil
}

// splitBySeparator cuts rawText into blocks at lines whose trimmed text equals
// the separator. Internal blank lines inside a block are preserved as
// paragraphs (only the block's leading/trailing blanks are trimmed by callers).
func splitBySeparator(rawText, separator string) []string {
	var blocks []string
	var block []string
	scanner := bufio.NewScanner(strings.NewReader(rawText))
	scanner.Buffer(make([]byte, 0, 64*1024), maxImportRawBytes)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == separator {
			blocks = append(blocks, strings.Join(block, "\n"))
			block = nil
			continue
		}
		block = append(block, line)
	}
	blocks = append(blocks, strings.Join(block, "\n"))
	return blocks
}

// ---------------------------------------------------------------------------
// CSV (UTF-8)
// ---------------------------------------------------------------------------

type csvImportParser struct{}

func (p *csvImportParser) Parse(_ context.Context, rawText string) ([]ParsedImportRow, map[string]any, error) {
	reader := csv.NewReader(strings.NewReader(rawText))
	reader.FieldsPerRecord = -1 // allow ragged rows; columns are matched by name
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return nil, nil, fmt.Errorf("csv is empty")
	}
	if err != nil {
		return nil, nil, fmt.Errorf("read csv header: %w", err)
	}
	cols, rawHeaders := normalizeCSVHeader(header)

	var rows []ParsedImportRow
	rowNumber := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("read csv row %d: %w", rowNumber+2, err)
		}
		rowNumber++
		row := ParsedImportRow{RowNumber: rowNumber}
		row.RawPayload = make(map[string]any, len(cols))
		lookup := func(name string) (any, bool) {
			for i, col := range cols {
				if col != name {
					continue
				}
				if i < len(record) {
					return record[i], true
				}
				return nil, true // column declared but cell empty
			}
			return nil, false
		}
		for i, col := range cols {
			if i < len(record) {
				row.RawPayload[col] = record[i]
			} else {
				row.RawPayload[col] = ""
			}
		}
		populateRow(&row, lookup)
		rows = append(rows, row)
	}

	mapping := make(map[string]any, len(cols))
	for i, col := range cols {
		if i < len(rawHeaders) {
			mapping[col] = rawHeaders[i]
		}
	}
	return rows, mapping, nil
}

// normalizeCSVHeader maps trimmed, lower-cased header cells to canonical field
// names (with a few Chinese aliases) so CSV imports are forgiving. Unknown
// headers pass through as their trimmed original so no data is silently dropped.
func normalizeCSVHeader(header []string) (cols []string, rawHeaders []string) {
	cols = make([]string, 0, len(header))
	rawHeaders = make([]string, 0, len(header))
	for _, cell := range header {
		raw := strings.TrimSpace(cell)
		key := strings.ToLower(raw)
		canonical, known := csvHeaderAliases[key]
		if !known {
			canonical = raw
		}
		cols = append(cols, canonical)
		rawHeaders = append(rawHeaders, raw)
	}
	return cols, rawHeaders
}

var csvHeaderAliases = map[string]string{
	"external_id": "external_id", "externalid": "external_id", "id": "external_id",
	"content": "content", "text": "content", "body": "content", "正文": "content",
	"title": "title", "标题": "title",
	"captured_at": "captured_at", "created_at": "captured_at", "datetime": "captured_at", "date": "captured_at", "时间": "captured_at",
	"timezone": "timezone", "时区": "timezone",
	"tags": "tags", "tag": "tags", "标签": "tags",
	"source_url": "source_url", "url": "source_url", "link": "source_url", "链接": "source_url",
	"location_name": "location_name", "location": "location_name", "place": "location_name", "地点": "location_name",
	"latitude": "latitude", "lat": "latitude", "纬度": "latitude",
	"longitude": "longitude", "lng": "longitude", "lon": "longitude", "经度": "longitude",
	"activity": "activity", "活动": "activity",
}

// ---------------------------------------------------------------------------
// JSONL
// ---------------------------------------------------------------------------

type jsonlImportParser struct{}

func (p *jsonlImportParser) Parse(_ context.Context, rawText string) ([]ParsedImportRow, map[string]any, error) {
	scanner := bufio.NewScanner(strings.NewReader(rawText))
	scanner.Buffer(make([]byte, 0, 64*1024), maxImportRawBytes)

	var rows []ParsedImportRow
	detected := map[string]bool{}
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		row := ParsedImportRow{RowNumber: len(rows) + 1}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			row.RawPayload = map[string]any{"_source_line": line}
			row.Errors = append(row.Errors, entity.ImportValidationError{
				Code:    "invalid_json",
				Message: fmt.Sprintf("第 %d 行不是合法 JSON: %v", lineNumber, err),
			})
			rows = append(rows, row)
			continue
		}
		for k := range obj {
			detected[k] = true
		}
		row.RawPayload = obj
		populateRow(&row, func(name string) (any, bool) {
			v, ok := obj[name]
			return v, ok
		})
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scan jsonl: %w", err)
	}

	mapping := make(map[string]any, len(detected))
	for k := range detected {
		mapping[k] = k
	}
	return rows, mapping, nil
}

// ---------------------------------------------------------------------------
// Shared field extraction
// ---------------------------------------------------------------------------

// populateRow fills the typed fields of a row from a name → value lookup and
// appends row-level validation errors. Content is required (G7 criterion 6
// keeps a missing time/place as unknown, but a row without content cannot
// become a capture).
func populateRow(row *ParsedImportRow, lookup func(name string) (any, bool)) {
	var errs []entity.ImportValidationError
	// Missing or unparsable time stays unknown (G7 criterion 6).
	row.CapturedAtPrecision = "unknown"

	if v, ok := lookup("content"); ok {
		if s, ok := asString(v); ok && strings.TrimSpace(s) != "" {
			row.Content = s
		} else {
			errs = append(errs, entity.ImportValidationError{Code: "missing_content", Message: "缺少内容"})
		}
	} else {
		errs = append(errs, entity.ImportValidationError{Code: "missing_content", Message: "缺少 content 列"})
	}

	if v, ok := lookup("external_id"); ok {
		if s, ok := asString(v); ok && s != "" {
			row.ExternalID = &s
		}
	}
	if v, ok := lookup("title"); ok {
		if s, ok := asString(v); ok && s != "" {
			row.Title = &s
		}
	}
	if v, ok := lookup("captured_at"); ok {
		if s, ok := asString(v); ok && s != "" {
			t, precision, err := parseCapturedAt(s)
			if err != nil {
				errs = append(errs, entity.ImportValidationError{Code: "invalid_date", Message: fmt.Sprintf("无法解析时间 %q", s)})
			} else {
				row.CapturedAt = t
				row.CapturedAtPrecision = precision
			}
		}
	}
	if v, ok := lookup("timezone"); ok {
		if s, ok := asString(v); ok && s != "" {
			row.Timezone = &s
		}
	}
	if v, ok := lookup("tags"); ok {
		row.Tags = parseTagsValue(v)
	}
	if v, ok := lookup("source_url"); ok {
		if s, ok := asString(v); ok && s != "" {
			row.SourceURL = &s
		}
	}
	if v, ok := lookup("location_name"); ok {
		if s, ok := asString(v); ok && s != "" {
			row.LocationName = &s
		}
	}
	if v, ok := lookup("activity"); ok {
		if s, ok := asString(v); ok && s != "" {
			row.Activity = &s
		}
	}

	latV, hasLat := lookup("latitude")
	lngV, hasLng := lookup("longitude")
	if hasLat || hasLng {
		latStr, latOK := asString(latV)
		lngStr, lngOK := asString(lngV)
		lat, latErr := parseCoordinate(latStr)
		lng, lngErr := parseCoordinate(lngStr)
		if !hasLat || !hasLng || !latOK || !lngOK || latErr != nil || lngErr != nil ||
			lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			errs = append(errs, entity.ImportValidationError{Code: "invalid_coordinates", Message: "经纬度必须成对且纬度∈[-90,90]、经度∈[-180,180]"})
		} else {
			row.Latitude = &lat
			row.Longitude = &lng
		}
	}

	row.Errors = errs
}

func asString(v any) (string, bool) {
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s), true
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(s), true
	default:
		return "", false
	}
}

func parseTagsValue(v any) []string {
	switch t := v.(type) {
	case string:
		return splitTags(t)
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := asString(item); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func splitTags(s string) []string {
	var out []string
	for _, part := range strings.Split(s, "|") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseCoordinate(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// parseCapturedAt parses ISO-8601-ish timestamps. RFC3339 and common local
// datetime layouts map to precision "exact"; a bare date maps to "date_only".
// It never fabricates a time from an import date (G7 criterion 6).
func parseCapturedAt(value string) (*time.Time, string, error) {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil, "unknown", nil
	}
	layouts := []struct {
		layout    string
		precision string
	}{
		{time.RFC3339Nano, "exact"},
		{time.RFC3339, "exact"},
		{"2006-01-02 15:04:05", "exact"},
		{"2006-01-02 15:04", "exact"},
		{"2006-01-02", "date_only"},
	}
	for _, l := range layouts {
		if t, err := time.Parse(l.layout, s); err == nil {
			return &t, l.precision, nil
		}
	}
	return nil, "", fmt.Errorf("invalid date %q", value)
}
