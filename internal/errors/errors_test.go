package errors

import (
	"errors"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := New(TypeInternal, "something went wrong")
	expected := "[INTERNAL] something went wrong"
	if err.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, err.Error())
	}
}

func TestAppError_Wrap(t *testing.T) {
	originalErr := errors.New("database connection refused")
	wrappedErr := Wrap(TypeInternal, "failed to connect to db", originalErr)

	expected := "[INTERNAL] failed to connect to db: database connection refused"
	if wrappedErr.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, wrappedErr.Error())
	}

	// Go'nun standart errors.Is fonsiyonu orijinal hatayı bulabilmeli (Unwrap testi)
	if !errors.Is(wrappedErr, originalErr) {
		t.Errorf("Wrapped error does not contain the original error")
	}
}

func TestIsType(t *testing.T) {
	err := New(TypeInvalidInput, "invalid email address")

	if !IsType(err, TypeInvalidInput) {
		t.Errorf("Expected true for TypeInvalidInput")
	}

	if IsType(err, TypeInternal) {
		t.Errorf("Expected false for TypeInternal")
	}

	// Sarmalanmış hatada da tipi doğru bulmalı
	wrappedErr := Wrap(TypeInvalidInput, "wrapping input error", err)
	if !IsType(wrappedErr, TypeInvalidInput) {
		t.Errorf("Expected true for wrapped TypeInvalidInput")
	}
}

func TestSentinelErrors(t *testing.T) {
	err := ErrConfigNotFound

	if !IsType(err, TypeNotFound) {
		t.Errorf("Expected ErrConfigNotFound to be TypeNotFound")
	}
}