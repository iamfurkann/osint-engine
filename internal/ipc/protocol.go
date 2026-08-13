package ipc

import (
	"encoding/json"
	"fmt"
)

// Request, IPC istek mesajıdır (JSON-RPC benzeri).
type Request struct {
	ID     string          `json:"id"`     // İstek ID'si (yanıtla eşleştirme)
	Method string          `json:"method"` // Çağrılacak handler metodu
	Params json.RawMessage `json:"params"` // Parametre verisi (JSON)
}

// Response, IPC yanıt mesajıdır.
type Response struct {
	ID     string          `json:"id"`               // Eşleşen istek ID'si
	Result json.RawMessage `json:"result,omitempty"` // Başarılı sonuç
	Error  string          `json:"error,omitempty"`  // Hata mesajı (varsa)
}

// HandlerFunc, bir IPC isteğini işleyen fonksiyon tipidir.
type HandlerFunc func(params json.RawMessage) (interface{}, error)

// NewErrorResponse, hata yanıtı oluşturur.
func NewErrorResponse(id string, err error) Response {
	return Response{ID: id, Error: err.Error()}
}

// NewSuccessResponse, başarılı yanıt oluşturur.
func NewSuccessResponse(id string, result interface{}) (Response, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return Response{}, fmt.Errorf("ipc: failed to marshal result: %w", err)
	}
	return Response{ID: id, Result: data}, nil
}

// MarshalParams, parametreleri JSON'a serialize eder.
func MarshalParams(v interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("ipc: failed to marshal params: %w", err)
	}
	return data, nil
}
