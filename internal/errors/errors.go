package errors

import (
	"errors"
	"fmt"
)

// ErrorType, hataları sınıflandırmak için kullanılır.
type ErrorType string

const (
	TypeInvalidInput ErrorType = "INVALID_INPUT"
	TypeNotFound     ErrorType = "NOT_FOUND"
	TypeInternal     ErrorType = "INTERNAL"
	TypeUnauthorized ErrorType = "UNAUTHORIZED"
	TypeTimeout      ErrorType = "TIMEOUT"
)

// AppError, sistem genelinde kullanılacak özel hata yapısıdır.
type AppError struct {
	Type    ErrorType // Hatanın sınıfı (örn: INVALID_INPUT)
	Message string    // Kullanıcıya gösterilecek dostane mesaj
	Err     error     // Hatanın kök nedeni (orijinal hata, opsiyonel)
}

// Error metodu, AppError'un Go'nun standart error arayüzünü (interface) uygulamasını sağlar.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap metodu, Go'nun errors.Is ve errors.As fonksiyonlarıyla çalışmasını sağlar.
func (e *AppError) Unwrap() error {
	return e.Err
}

// New, belirtilen tip ve mesajda kök nedeni olmayan yeni bir hata oluşturur.
func New(t ErrorType, msg string) error {
	return &AppError{
		Type:    t,
		Message: msg,
	}
}

// Wrap, mevcut bir hatayı (err) kendi sistemimize uygun şekilde sarmalar (wrap eder).
func Wrap(t ErrorType, msg string, err error) error {
	return &AppError{
		Type:    t,
		Message: msg,
		Err:     err,
	}
}

// IsType, bir hatanın belirli bir ErrorType'a ait olup olmadığını kontrol eder.
func IsType(err error, t ErrorType) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == t
	}
	return false
}

// Yaygın Sentinel (Standart) Hatalar
// Bunlar projenin herhangi bir yerinde doğrudan return edilebilecek kalıp hatalardır.
var (
	ErrConfigNotFound = New(TypeNotFound, "configuration file not found")
	ErrInvalidAPIKey  = New(TypeUnauthorized, "invalid or missing API key")
	ErrRateLimit      = New(TypeTimeout, "rate limit exceeded")
)