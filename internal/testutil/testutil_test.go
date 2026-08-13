package testutil

import (
	"os"
	"strings"
	"testing"
)

func TestSetEnv(t *testing.T) {
	// Önce sistemde olmayan bir değişkeni test edelim
	key := "OSINT_TEST_DUMMY_ENV"

	SetEnv(t, key, "hello_world")

	if val := os.Getenv(key); val != "hello_world" {
		t.Errorf("Expected hello_world, got %s", val)
	}

	// Not: Cleanup kısmı Go'nun test runner'ı tarafından test bitiminde otomatik çağrılır,
	// bu yüzden burada manuel bir şey yapmamıza gerek yok.
}

func TestTempSQLiteDBPath(t *testing.T) {
	dbPath := TempSQLiteDBPath(t)

	if !strings.HasSuffix(dbPath, "osint_test.db") {
		t.Errorf("Expected db path to end with osint_test.db, got %s", dbPath)
	}
}

func TestLoadFixture(t *testing.T) {
	data := LoadFixture(t, "mock_response.json")

	if len(data) == 0 {
		t.Error("Expected fixture data to not be empty")
	}

	content := string(data)
	if !strings.Contains(content, "test_data") {
		t.Errorf("Expected fixture to contain 'test_data', got %s", content)
	}
}
