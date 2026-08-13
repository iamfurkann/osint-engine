package engine

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/iamfurkann/osint-engine/pkg/plugin"
)

// parseSemVer, "v1.2.3" veya "1.2.3" formatındaki sürüm stringinden major, minor, patch döndürür.
// Pre-release ve build metadata görmezden gelinir (karşılaştırma için yeterli).
func parseSemVer(version string) (major, minor, patch int, err error) {
	v := strings.TrimPrefix(version, "v")

	// Pre-release (-beta) ve build metadata (+build.123) kısmını kes
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid semver format: %q (expected X.Y.Z)", version)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version in %q: %w", version, err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version in %q: %w", version, err)
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version in %q: %w", version, err)
	}

	return major, minor, patch, nil
}

// isVersionGTE, current sürümünün required sürümünden büyük veya eşit olup olmadığını kontrol eder.
// current >= required ise true döner.
func isVersionGTE(current, required string) (bool, error) {
	cMajor, cMinor, cPatch, err := parseSemVer(current)
	if err != nil {
		return false, fmt.Errorf("engine version: %w", err)
	}

	rMajor, rMinor, rPatch, err := parseSemVer(required)
	if err != nil {
		return false, fmt.Errorf("required version: %w", err)
	}

	if cMajor != rMajor {
		return cMajor > rMajor, nil
	}
	if cMinor != rMinor {
		return cMinor > rMinor, nil
	}
	return cPatch >= rPatch, nil
}

// CheckCompatibility, engine sürümü ile plugin manifest'inin gerektirdiği minimum sürümü
// karşılaştırır. EngineMinVersion boşsa plugin her zaman uyumlu kabul edilir.
// Uyumsuzsa açık bir hata mesajı döner.
func CheckCompatibility(engineVersion string, m plugin.Manifest) error {
	// EngineMinVersion belirtilmemişse her zaman uyumlu
	if strings.TrimSpace(m.EngineMinVersion) == "" {
		return nil
	}

	compatible, err := isVersionGTE(engineVersion, m.EngineMinVersion)
	if err != nil {
		return fmt.Errorf("version check failed for plugin %q: %w", m.Name, err)
	}

	if !compatible {
		return fmt.Errorf(
			"plugin %q requires engine %s or newer, but current engine version is %s",
			m.Name, m.EngineMinVersion, engineVersion,
		)
	}

	return nil
}
