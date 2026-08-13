package engine

import (
	"testing"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

func TestCheckCompatibility_Compatible(t *testing.T) {
	cases := []struct {
		engine  string
		minVer  string
		comment string
	}{
		{"v1.0.0", "v1.0.0", "exact match"},
		{"v2.0.0", "v1.0.0", "major ahead"},
		{"v1.5.0", "v1.0.0", "minor ahead"},
		{"v1.0.1", "v1.0.0", "patch ahead"},
		{"v1.2.3", "v0.9.0", "all components ahead"},
	}

	for _, c := range cases {
		m := plugin.Manifest{
			Name:             "test",
			EngineMinVersion: c.minVer,
		}
		err := CheckCompatibility(c.engine, m)
		if err != nil {
			t.Errorf("expected compatible (%s): engine=%s, min=%s, got: %v", c.comment, c.engine, c.minVer, err)
		}
	}
}

func TestCheckCompatibility_Incompatible(t *testing.T) {
	cases := []struct {
		engine  string
		minVer  string
		comment string
	}{
		{"v0.9.0", "v1.0.0", "major behind"},
		{"v1.0.0", "v1.1.0", "minor behind"},
		{"v1.0.0", "v1.0.1", "patch behind"},
		{"v0.0.1", "v2.0.0", "far behind"},
	}

	for _, c := range cases {
		m := plugin.Manifest{
			Name:             "test",
			EngineMinVersion: c.minVer,
		}
		err := CheckCompatibility(c.engine, m)
		if err == nil {
			t.Errorf("expected incompatible (%s): engine=%s, min=%s, got nil", c.comment, c.engine, c.minVer)
		}
	}
}

func TestCheckCompatibility_EmptyMinVersion(t *testing.T) {
	m := plugin.Manifest{
		Name:             "test",
		EngineMinVersion: "",
	}
	err := CheckCompatibility("v0.0.1", m)
	if err != nil {
		t.Errorf("expected no error for empty EngineMinVersion, got: %v", err)
	}
}

func TestCheckCompatibility_PreReleaseIgnored(t *testing.T) {
	// Pre-release kısmı karşılaştırmada görmezden gelinmeli
	m := plugin.Manifest{
		Name:             "test",
		EngineMinVersion: "v1.0.0-beta",
	}
	err := CheckCompatibility("v1.0.0", m)
	if err != nil {
		t.Errorf("expected compatible (pre-release ignored), got: %v", err)
	}
}

func TestParseSemVer_Valid(t *testing.T) {
	cases := []struct {
		input               string
		major, minor, patch int
	}{
		{"v1.2.3", 1, 2, 3},
		{"0.1.0", 0, 1, 0},
		{"v10.20.30", 10, 20, 30},
		{"v1.0.0-beta+build.123", 1, 0, 0},
	}

	for _, c := range cases {
		major, minor, patch, err := parseSemVer(c.input)
		if err != nil {
			t.Errorf("parseSemVer(%q) failed: %v", c.input, err)
			continue
		}
		if major != c.major || minor != c.minor || patch != c.patch {
			t.Errorf("parseSemVer(%q) = %d.%d.%d, want %d.%d.%d", c.input, major, minor, patch, c.major, c.minor, c.patch)
		}
	}
}

func TestParseSemVer_Invalid(t *testing.T) {
	cases := []string{"abc", "1.2", "v1", "latest", ""}
	for _, v := range cases {
		_, _, _, err := parseSemVer(v)
		if err == nil {
			t.Errorf("expected error for parseSemVer(%q), got nil", v)
		}
	}
}
