package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

// Client, Unix Domain Socket üzerinden daemon ile iletişim kuran istemcidir.
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient, yeni bir IPC istemcisi oluşturur.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		timeout:    10 * time.Second,
	}
}

// Call, daemon'a bir metot çağrısı yapar ve yanıtı döner.
func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("ipc: daemon'a bağlanılamadı (%s): %w — daemon çalışıyor mu? (osintd start)", c.socketPath, err)
	}
	defer conn.Close()

	// Timeout ayarla
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("ipc: deadline ayarlanamadı: %w", err)
	}

	// Request oluştur
	rawParams, err := MarshalParams(params)
	if err != nil {
		return nil, err
	}

	req := Request{
		ID:     uuid.New().String(),
		Method: method,
		Params: rawParams,
	}

	// İsteği gönder (JSON + newline)
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("ipc: request marshal failed: %w", err)
	}
	data = append(data, '\n')

	if _, err := conn.Write(data); err != nil {
		return nil, fmt.Errorf("ipc: request write failed: %w", err)
	}

	// Yanıtı oku
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("ipc: response read failed: %w", err)
		}
		return nil, fmt.Errorf("ipc: empty response from daemon")
	}

	var resp Response
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("ipc: response parse failed: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}

	return resp.Result, nil
}

// Ping, daemon'ın çalışıp çalışmadığını kontrol eder.
func (c *Client) Ping() error {
	_, err := c.Call("ping", nil)
	return err
}

// IsRunning, daemon'ın erişilebilir olup olmadığını kontrol eder.
func (c *Client) IsRunning() bool {
	return c.Ping() == nil
}
