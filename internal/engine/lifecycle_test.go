package engine

import (
	"fmt"
	"testing"
)

func TestLifecycle_ActivateDeactivate(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())

	// Register sonrası inactive olmalı
	status, err := lm.GetStatus("dummy-module")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.State != StateInactive {
		t.Errorf("expected state inactive after register, got %q", status.State)
	}

	// Activate
	if err := lm.Activate("dummy-module"); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}
	status, _ = lm.GetStatus("dummy-module")
	if status.State != StateActive {
		t.Errorf("expected state active, got %q", status.State)
	}
	if status.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set after activation")
	}

	// Tekrar Activate → idempotent, hata olmamalı
	if err := lm.Activate("dummy-module"); err != nil {
		t.Errorf("expected no error for double activate, got: %v", err)
	}

	// Deactivate
	if err := lm.Deactivate("dummy-module"); err != nil {
		t.Fatalf("Deactivate failed: %v", err)
	}
	status, _ = lm.GetStatus("dummy-module")
	if status.State != StateInactive {
		t.Errorf("expected state inactive after deactivate, got %q", status.State)
	}
	if status.StoppedAt.IsZero() {
		t.Error("expected StoppedAt to be set after deactivation")
	}

	// Tekrar Deactivate → idempotent
	if err := lm.Deactivate("dummy-module"); err != nil {
		t.Errorf("expected no error for double deactivate, got: %v", err)
	}
}

func TestLifecycle_MarkError(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())
	_ = lm.Activate("dummy-module")

	// TEK hata plugin'i devre dışı BIRAKMAMALI.
	// Aksi hâlde crt.sh'den gelen geçici bir 502, connector'ı daemon ömrü
	// boyunca öldürür (üretimde Restart() hiç çağrılmıyor).
	testErr := fmt.Errorf("connection timeout")
	lm.MarkError("dummy-module", testErr)

	status, _ := lm.GetStatus("dummy-module")
	if status.State != StateActive {
		t.Errorf("tek hatadan sonra hâlâ active olmalı, alınan %q", status.State)
	}
	if status.ErrorCount != 1 {
		t.Errorf("expected error count 1, got %d", status.ErrorCount)
	}
	if status.LastError == nil || status.LastError.Error() != "connection timeout" {
		t.Errorf("expected last error 'connection timeout', got %v", status.LastError)
	}

	// Eşiğe kadar aktif kalmalı
	lm.MarkError("dummy-module", fmt.Errorf("second error"))
	status, _ = lm.GetStatus("dummy-module")
	if status.ErrorCount != 2 {
		t.Errorf("expected error count 2, got %d", status.ErrorCount)
	}
	if status.State != StateActive {
		t.Errorf("eşik altında active kalmalı, alınan %q", status.State)
	}

	// Eşiğe ulaşınca izole edilmeli
	lm.MarkError("dummy-module", fmt.Errorf("third error"))
	status, _ = lm.GetStatus("dummy-module")
	if status.State != StateError {
		t.Errorf("eşikte (%d) error olmalı, alınan %q", maxConsecutiveErrors, status.State)
	}
}

// TestLifecycle_MarkSuccessResetsErrors, başarılı çalıştırmanın ardışık hata
// sayacını sıfırladığını doğrular. Bu olmadan eşik, uzun süre çalışan bir
// daemon'da birbirinden bağımsız aralıklı hataların birikmesiyle dolardı.
func TestLifecycle_MarkSuccessResetsErrors(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())
	_ = lm.Activate("dummy-module")

	// Eşiğin bir altına kadar hata biriktir
	for i := 0; i < maxConsecutiveErrors-1; i++ {
		lm.MarkError("dummy-module", fmt.Errorf("transient %d", i))
	}

	lm.MarkSuccess("dummy-module")

	status, _ := lm.GetStatus("dummy-module")
	if status.ErrorCount != 0 {
		t.Errorf("başarı sayacı sıfırlamalı, alınan %d", status.ErrorCount)
	}

	// Sıfırlandığı için bir hata daha plugin'i öldürmemeli
	lm.MarkError("dummy-module", fmt.Errorf("after reset"))
	status, _ = lm.GetStatus("dummy-module")
	if status.State != StateActive {
		t.Errorf("sıfırlama sonrası tek hata izole etmemeli, alınan %q", status.State)
	}
}

func TestLifecycle_Restart(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())
	_ = lm.Activate("dummy-module")

	// Restart yalnızca error durumundan yapılabilir; plugin'i eşiğe kadar sür.
	for i := 0; i < maxConsecutiveErrors; i++ {
		lm.MarkError("dummy-module", fmt.Errorf("test error %d", i))
	}

	// Error durumundan restart
	if err := lm.Restart("dummy-module"); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}

	status, _ := lm.GetStatus("dummy-module")
	if status.State != StateActive {
		t.Errorf("expected state active after restart, got %q", status.State)
	}

	// ErrorCount sıfırlanMAMALI (tarihsel takip)
	if status.ErrorCount != maxConsecutiveErrors {
		t.Errorf("expected error count %d (preserved), got %d", maxConsecutiveErrors, status.ErrorCount)
	}

	// Aktif plugin'i restart etmeye çalışmak hata vermeli
	err := lm.Restart("dummy-module")
	if err == nil {
		t.Fatal("expected error when restarting active plugin, got nil")
	}
}

func TestLifecycle_IsActive(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())

	// Kayıtlı ama inactive → false
	if lm.IsActive("dummy-module") {
		t.Error("expected IsActive false for inactive plugin")
	}

	_ = lm.Activate("dummy-module")
	if !lm.IsActive("dummy-module") {
		t.Error("expected IsActive true for active plugin")
	}

	// Tek hata izole ETMEMELİ
	lm.MarkError("dummy-module", fmt.Errorf("transient"))
	if !lm.IsActive("dummy-module") {
		t.Error("tek geçici hatadan sonra plugin hâlâ aktif olmalı")
	}

	// Eşiğe ulaşınca izole edilmeli
	for i := 1; i < maxConsecutiveErrors; i++ {
		lm.MarkError("dummy-module", fmt.Errorf("err %d", i))
	}
	if lm.IsActive("dummy-module") {
		t.Error("expected IsActive false for error plugin")
	}

	// Takip edilmeyen plugin → false
	if lm.IsActive("nonexistent") {
		t.Error("expected IsActive false for nonexistent plugin")
	}
}

func TestLifecycle_ActivateAll(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())
	_ = registry.Register(&analyzerModule{})

	lm.ActivateAll()

	if !lm.IsActive("dummy-module") {
		t.Error("expected dummy-module active after ActivateAll")
	}
	if !lm.IsActive("test-analyzer") {
		t.Error("expected test-analyzer active after ActivateAll")
	}
}

func TestLifecycle_DeactivateAll(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())
	lm.ActivateAll()
	lm.DeactivateAll()

	if lm.IsActive("dummy-module") {
		t.Error("expected dummy-module inactive after DeactivateAll")
	}
}

func TestLifecycle_NonexistentPlugin(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)

	err := lm.Activate("nonexistent")
	if err == nil {
		t.Fatal("expected error for activating nonexistent plugin, got nil")
	}

	err = lm.Deactivate("nonexistent")
	if err == nil {
		t.Fatal("expected error for deactivating nonexistent plugin, got nil")
	}

	_, err = lm.GetStatus("nonexistent")
	if err == nil {
		t.Fatal("expected error for getting status of nonexistent plugin, got nil")
	}
}

func TestLifecycle_UnregisterCleansUpStatus(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())
	_ = lm.Activate("dummy-module")

	// Unregister → lifecycle'dan da temizlenmeli
	_ = registry.Unregister("dummy-module")

	_, err := lm.GetStatus("dummy-module")
	if err == nil {
		t.Fatal("expected error after unregister, plugin should be untracked")
	}
}

func TestLifecycle_ListStatuses(t *testing.T) {
	registry := NewRegistry()
	lm := NewLifecycleManager(registry)
	registry.SetLifecycle(lm)

	_ = registry.Register(NewDummyModule())
	_ = registry.Register(&analyzerModule{})
	_ = lm.Activate("dummy-module")

	statuses := lm.ListStatuses()
	if len(statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(statuses))
	}

	if statuses["dummy-module"].State != StateActive {
		t.Errorf("expected dummy-module active, got %q", statuses["dummy-module"].State)
	}
	if statuses["test-analyzer"].State != StateInactive {
		t.Errorf("expected test-analyzer inactive, got %q", statuses["test-analyzer"].State)
	}
}
