package input

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// InputType, kullanıcı girdisinin tespit edilen tipini temsil eder.
type InputType string

const (
	TypeEmail    InputType = "email"
	TypeDomain   InputType = "domain"
	TypeIP       InputType = "ip"
	TypeUsername InputType = "username"
	TypeURL      InputType = "url"
	TypeHash     InputType = "hash"
	TypeUnknown  InputType = "unknown"
)

// Regex pattern'ler (derleme zamanında bir kez compile edilir)
var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	domainRegex   = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
	md5Regex      = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
	sha1Regex     = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	sha256Regex   = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.\-]{3,39}$`)
)

// Detect, verilen string'in tipini otomatik olarak algılar.
// Algılama sırası: URL → Email → IP → Hash → Domain → Username → Unknown
func Detect(value string) InputType {
	value = strings.TrimSpace(value)
	if value == "" {
		return TypeUnknown
	}

	// 1. URL kontrolü (http:// veya https:// ile başlıyorsa)
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		if _, err := url.ParseRequestURI(value); err == nil {
			return TypeURL
		}
	}

	// 2. Email kontrolü
	if emailRegex.MatchString(value) {
		return TypeEmail
	}

	// 3. IP kontrolü (IPv4 veya IPv6)
	if ip := net.ParseIP(value); ip != nil {
		return TypeIP
	}

	// 4. Hash kontrolü (MD5, SHA1, SHA256)
	if md5Regex.MatchString(value) || sha1Regex.MatchString(value) || sha256Regex.MatchString(value) {
		return TypeHash
	}

	// 5. Domain kontrolü
	if domainRegex.MatchString(value) && strings.Contains(value, ".") {
		return TypeDomain
	}

	// 6. Username kontrolü
	if usernameRegex.MatchString(value) {
		return TypeUsername
	}

	return TypeUnknown
}

// Validate, verilen değerin belirtilen tipte geçerli olup olmadığını kontrol eder.
// Geçersizse açıklayıcı hata mesajı döner.
func Validate(value string, expectedType InputType) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("input validation: value cannot be empty")
	}

	switch expectedType {
	case TypeEmail:
		if !emailRegex.MatchString(value) {
			return fmt.Errorf("input validation: %q is not a valid email address", value)
		}
	case TypeDomain:
		if !domainRegex.MatchString(value) {
			return fmt.Errorf("input validation: %q is not a valid domain name", value)
		}
	case TypeIP:
		if net.ParseIP(value) == nil {
			return fmt.Errorf("input validation: %q is not a valid IP address", value)
		}
	case TypeUsername:
		if !usernameRegex.MatchString(value) {
			return fmt.Errorf("input validation: %q is not a valid username (3-39 chars, alphanumeric/._-)", value)
		}
	case TypeURL:
		if _, err := url.ParseRequestURI(value); err != nil {
			return fmt.Errorf("input validation: %q is not a valid URL", value)
		}
	case TypeHash:
		if !md5Regex.MatchString(value) && !sha1Regex.MatchString(value) && !sha256Regex.MatchString(value) {
			return fmt.Errorf("input validation: %q is not a valid hash (MD5/SHA1/SHA256)", value)
		}
	default:
		return fmt.Errorf("input validation: unknown type %q", expectedType)
	}

	return nil
}

// AllTypes, desteklenen tüm girdi tiplerini döndürür (CLI yardım metni için).
func AllTypes() []InputType {
	return []InputType{TypeEmail, TypeDomain, TypeIP, TypeUsername, TypeURL, TypeHash}
}
