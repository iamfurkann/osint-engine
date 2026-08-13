package domain

import (
	"strings"
	"testing"
)

// --- Investigation.Validate() Testleri ---

func TestInvestigationValidate_Valid(t *testing.T) {
	inv := &Investigation{
		ID:     "inv-001",
		Name:   "Target Corp Investigation",
		Status: StatusActive,
	}

	if err := inv.Validate(); err != nil {
		t.Errorf("expected no error for valid investigation, got: %v", err)
	}
}

func TestInvestigationValidate_EmptyID(t *testing.T) {
	inv := &Investigation{ID: "", Name: "Test", Status: StatusActive}
	err := inv.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
	if !strings.Contains(err.Error(), "ID") {
		t.Errorf("error should mention 'ID', got: %v", err)
	}
}

func TestInvestigationValidate_EmptyName(t *testing.T) {
	inv := &Investigation{ID: "inv-1", Name: "   ", Status: StatusActive}
	err := inv.Validate()
	if err == nil {
		t.Fatal("expected error for whitespace-only name, got nil")
	}
}

func TestInvestigationValidate_NameTooLong(t *testing.T) {
	longName := strings.Repeat("a", 256)
	inv := &Investigation{ID: "inv-1", Name: longName, Status: StatusActive}
	err := inv.Validate()
	if err == nil {
		t.Fatal("expected error for name exceeding 255 chars, got nil")
	}
}

func TestInvestigationValidate_InvalidStatus(t *testing.T) {
	inv := &Investigation{ID: "inv-1", Name: "Test", Status: "running"}
	err := inv.Validate()
	if err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
}

func TestInvestigationValidate_AllValidStatuses(t *testing.T) {
	statuses := []InvestigationStatus{StatusActive, StatusPaused, StatusCompleted, StatusArchived}
	for _, s := range statuses {
		inv := &Investigation{ID: "inv-1", Name: "Test", Status: s}
		if err := inv.Validate(); err != nil {
			t.Errorf("expected no error for status %q, got: %v", s, err)
		}
	}
}

// --- Plugin.Validate() Testleri ---

func TestPluginValidate_Valid(t *testing.T) {
	p := &Plugin{
		Name:     "github-scraper",
		Version:  "v1.0.0",
		Status:   PluginStatusActive,
		Language: "go",
	}

	if err := p.Validate(); err != nil {
		t.Errorf("expected no error for valid plugin, got: %v", err)
	}
}

func TestPluginValidate_EmptyName(t *testing.T) {
	p := &Plugin{Name: "", Version: "v1.0.0", Status: PluginStatusActive, Language: "go"}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestPluginValidate_NameTooLong(t *testing.T) {
	p := &Plugin{
		Name:     strings.Repeat("x", 101),
		Version:  "v1.0.0",
		Status:   PluginStatusActive,
		Language: "go",
	}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for name exceeding 100 chars, got nil")
	}
}

func TestPluginValidate_InvalidVersion(t *testing.T) {
	cases := []string{"abc", "1.2", "v1", "latest"}
	for _, ver := range cases {
		p := &Plugin{Name: "test", Version: ver, Status: PluginStatusActive, Language: "go"}
		if err := p.Validate(); err == nil {
			t.Errorf("expected error for invalid version %q, got nil", ver)
		}
	}
}

func TestPluginValidate_ValidVersionFormats(t *testing.T) {
	cases := []string{"v1.0.0", "0.1.0", "v2.3.4-beta", "1.0.0-rc.1"}
	for _, ver := range cases {
		p := &Plugin{Name: "test", Version: ver, Status: PluginStatusActive, Language: "go"}
		if err := p.Validate(); err != nil {
			t.Errorf("expected no error for version %q, got: %v", ver, err)
		}
	}
}

func TestPluginValidate_EmptyLanguage(t *testing.T) {
	p := &Plugin{Name: "test", Version: "v1.0.0", Status: PluginStatusActive, Language: ""}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for empty language, got nil")
	}
}

func TestPluginValidate_InvalidStatus(t *testing.T) {
	p := &Plugin{Name: "test", Version: "v1.0.0", Status: "broken", Language: "go"}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for invalid status, got nil")
	}
}

func TestPluginValidate_AllValidStatuses(t *testing.T) {
	statuses := []PluginStatus{PluginStatusActive, PluginStatusInactive, PluginStatusError}
	for _, s := range statuses {
		p := &Plugin{Name: "test", Version: "v1.0.0", Status: s, Language: "go"}
		if err := p.Validate(); err != nil {
			t.Errorf("expected no error for status %q, got: %v", s, err)
		}
	}
}
