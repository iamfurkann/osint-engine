package ipc

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestIPC_PingPong(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	server := NewServer(socketPath)
	server.RegisterHandler("ping", func(params json.RawMessage) (interface{}, error) {
		return map[string]string{"status": "pong"}, nil
	})

	if err := server.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer server.Shutdown()

	// Client bağlan
	time.Sleep(50 * time.Millisecond) // Socket hazır olsun
	client := NewClient(socketPath)

	result, err := client.Call("ping", nil)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if resp["status"] != "pong" {
		t.Errorf("expected 'pong', got %q", resp["status"])
	}
}

func TestIPC_UnknownMethod(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	server := NewServer(socketPath)
	if err := server.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer server.Shutdown()

	time.Sleep(50 * time.Millisecond)
	client := NewClient(socketPath)

	_, err := client.Call("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
}

func TestIPC_WithParams(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	type EchoParams struct {
		Message string `json:"message"`
	}

	server := NewServer(socketPath)
	server.RegisterHandler("echo", func(params json.RawMessage) (interface{}, error) {
		var p EchoParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return map[string]string{"echo": p.Message}, nil
	})

	if err := server.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer server.Shutdown()

	time.Sleep(50 * time.Millisecond)
	client := NewClient(socketPath)

	result, err := client.Call("echo", EchoParams{Message: "hello"})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp["echo"] != "hello" {
		t.Errorf("expected 'hello', got %q", resp["echo"])
	}
}

func TestIPC_MultipleCalls(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	counter := 0
	server := NewServer(socketPath)
	server.RegisterHandler("increment", func(params json.RawMessage) (interface{}, error) {
		counter++
		return map[string]int{"count": counter}, nil
	})

	if err := server.Listen(); err != nil {
		t.Fatalf("Listen failed: %v", err)
	}
	defer server.Shutdown()

	time.Sleep(50 * time.Millisecond)

	// 5 ayrı istemci bağlantısı
	for i := 0; i < 5; i++ {
		client := NewClient(socketPath)
		_, err := client.Call("increment", nil)
		if err != nil {
			t.Fatalf("Call %d failed: %v", i, err)
		}
	}

	if counter != 5 {
		t.Errorf("expected counter=5, got %d", counter)
	}
}

func TestIPC_IsRunning(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")

	// Daemon yok — bağlanamamalı
	client := NewClient(socketPath)
	if client.IsRunning() {
		t.Error("expected IsRunning=false when no server")
	}

	// Sunucu başlat
	server := NewServer(socketPath)
	server.RegisterHandler("ping", func(params json.RawMessage) (interface{}, error) {
		return "pong", nil
	})
	if err := server.Listen(); err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer server.Shutdown()

	time.Sleep(50 * time.Millisecond)

	if !client.IsRunning() {
		t.Error("expected IsRunning=true when server is running")
	}
}

func TestProtocol_MarshalParams(t *testing.T) {
	data, err := MarshalParams(map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("MarshalParams failed: %v", err)
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m["key"] != "value" {
		t.Errorf("expected 'value', got %q", m["key"])
	}
}
