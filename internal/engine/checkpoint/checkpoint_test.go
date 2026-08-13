package checkpoint

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckpointManager_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir, 1*time.Minute)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	invID := "inv-123"
	cp := &Checkpoint{
		InvestigationID: invID,
		CompletedTasks:  []string{"task-1", "task-2"},
		Progress:        50.0,
	}

	m.Update(cp)
	if err := m.Save(invID); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Yeni bir manager ile yükle (diskte kayıtlı mı kontrolü)
	m2, _ := NewManager(dir, 1*time.Minute)
	loaded, err := m2.Load(invID)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.InvestigationID != invID {
		t.Errorf("expected invID %q, got %q", invID, loaded.InvestigationID)
	}
	if len(loaded.CompletedTasks) != 2 {
		t.Errorf("expected 2 completed tasks, got %d", len(loaded.CompletedTasks))
	}
	if loaded.Progress != 50.0 {
		t.Errorf("expected progress 50.0, got %f", loaded.Progress)
	}
}

func TestCheckpointManager_MarkTaskCompleted(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, 1*time.Minute)

	invID := "inv-test"
	m.MarkTaskCompleted(invID, "task-1")
	m.MarkTaskCompleted(invID, "task-2")
	m.MarkTaskCompleted(invID, "task-1") // Duplicate

	if err := m.Save(invID); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	cp, err := m.Load(invID) // Memory'den okur
	if err != nil {
		t.Fatalf("Load from memory failed: %v", err)
	}

	if len(cp.CompletedTasks) != 2 {
		t.Errorf("expected 2 unique completed tasks, got %d", len(cp.CompletedTasks))
	}
}

func TestCheckpointManager_RecoverAll(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, 1*time.Minute)

	m.Update(&Checkpoint{InvestigationID: "inv-1"})
	m.Update(&Checkpoint{InvestigationID: "inv-2"})
	m.SaveAll()

	// Yeni manager ile recover
	m2, _ := NewManager(dir, 1*time.Minute)
	recovered, err := m2.RecoverAll()
	if err != nil {
		t.Fatalf("RecoverAll failed: %v", err)
	}

	if len(recovered) != 2 {
		t.Errorf("expected 2 recovered checkpoints, got %d", len(recovered))
	}

	// Hafızaya alınmış mı?
	_, err = m2.Load("inv-1")
	if err != nil {
		t.Error("expected inv-1 to be loaded in memory after recovery")
	}
}

func TestCheckpointManager_Remove(t *testing.T) {
	dir := t.TempDir()
	m, _ := NewManager(dir, 1*time.Minute)

	invID := "inv-delete"
	m.Update(&Checkpoint{InvestigationID: invID})
	m.SaveAll()

	m.Remove(invID)

	// Dosya silindi mi?
	_, err := os.Stat(filepath.Join(dir, invID+".json"))
	if !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}

	// Memory'den silindi mi?
	_, err = m.Load(invID)
	if err == nil {
		t.Error("expected Load to fail after Remove")
	}
}
