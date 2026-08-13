package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/iamfurkann/osint-engine/internal/domain"
)

func sampleFindings() []domain.Finding {
	return []domain.Finding{
		{
			ID:              "f1",
			InvestigationID: "inv-1",
			Type:            "email",
			Value:           "admin@example.com",
			FoundBy:         "mock-connector",
			Context:         `{"source":"test"}`,
			CreatedAt:       time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			ID:              "f2",
			InvestigationID: "inv-1",
			Type:            "ip",
			Value:           "93.184.216.34",
			FoundBy:         "dns-resolver",
			Context:         "{}",
			CreatedAt:       time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC),
		},
	}
}

func TestTableFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{}

	if err := f.Render(&buf, sampleFindings()); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "admin@example.com") {
		t.Error("table should contain email value")
	}
	if !strings.Contains(out, "mock-connector") {
		t.Error("table should contain source name")
	}
}

func TestTableFormatter_Empty(t *testing.T) {
	var buf bytes.Buffer
	f := &TableFormatter{}
	_ = f.Render(&buf, []domain.Finding{})

	if !strings.Contains(buf.String(), "Sonuç bulunamadı") {
		t.Error("expected 'no results' message for empty findings")
	}
}

func TestJSONFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{}

	if err := f.Render(&buf, sampleFindings()); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"admin@example.com"`) {
		t.Error("JSON should contain email value")
	}
	if !strings.Contains(out, `"email"`) {
		t.Error("JSON should contain type field")
	}
}

func TestCSVFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &CSVFormatter{}

	if err := f.Render(&buf, sampleFindings()); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 { // header + 2 data rows
		t.Errorf("expected 3 lines (header + 2 rows), got %d", len(lines))
	}
	if !strings.Contains(lines[0], "ID,InvestigationID,Type") {
		t.Error("CSV should have header row")
	}
}

func TestParseFormat(t *testing.T) {
	cases := []struct {
		input    string
		expected Format
		hasError bool
	}{
		{"table", FormatTable, false},
		{"json", FormatJSON, false},
		{"csv", FormatCSV, false},
		{"JSON", FormatJSON, false}, // case insensitive
		{"", FormatTable, false},    // default
		{"xml", "", true},           // unsupported
	}

	for _, c := range cases {
		got, err := ParseFormat(c.input)
		if c.hasError && err == nil {
			t.Errorf("ParseFormat(%q) expected error, got nil", c.input)
		}
		if !c.hasError && err != nil {
			t.Errorf("ParseFormat(%q) unexpected error: %v", c.input, err)
		}
		if !c.hasError && got != c.expected {
			t.Errorf("ParseFormat(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestNewFormatter(t *testing.T) {
	table := NewFormatter(FormatTable)
	if _, ok := table.(*TableFormatter); !ok {
		t.Error("expected TableFormatter")
	}

	jsonF := NewFormatter(FormatJSON)
	if _, ok := jsonF.(*JSONFormatter); !ok {
		t.Error("expected JSONFormatter")
	}

	csvF := NewFormatter(FormatCSV)
	if _, ok := csvF.(*CSVFormatter); !ok {
		t.Error("expected CSVFormatter")
	}
}
