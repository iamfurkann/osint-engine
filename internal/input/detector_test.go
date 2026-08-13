package input

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		input    string
		expected InputType
	}{
		// Email
		{"user@example.com", TypeEmail},
		{"john.doe+tag@sub.domain.co.uk", TypeEmail},

		// Domain
		{"example.com", TypeDomain},
		{"sub.example.co.uk", TypeDomain},
		{"api.github.com", TypeDomain},

		// IP
		{"8.8.8.8", TypeIP},
		{"192.168.1.1", TypeIP},
		{"::1", TypeIP},
		{"2001:db8::1", TypeIP},

		// URL
		{"https://example.com/path?q=1", TypeURL},
		{"http://api.github.com", TypeURL},

		// Hash
		{"d41d8cd98f00b204e9800998ecf8427e", TypeHash},                                 // MD5
		{"da39a3ee5e6b4b0d3255bfef95601890afd80709", TypeHash},                         // SHA1
		{"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", TypeHash}, // SHA256

		// Username
		{"johndoe", TypeUsername},
		{"user_name", TypeUsername},
		{"user.name-123", TypeUsername},

		// Unknown
		{"", TypeUnknown},
		{"ab", TypeUnknown},          // çok kısa username
		{"hello world", TypeUnknown}, // boşluk var
	}

	for _, c := range cases {
		got := Detect(c.input)
		if got != c.expected {
			t.Errorf("Detect(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestValidate_Valid(t *testing.T) {
	cases := []struct {
		value string
		typ   InputType
	}{
		{"user@example.com", TypeEmail},
		{"example.com", TypeDomain},
		{"8.8.8.8", TypeIP},
		{"johndoe", TypeUsername},
		{"https://example.com", TypeURL},
		{"d41d8cd98f00b204e9800998ecf8427e", TypeHash},
	}

	for _, c := range cases {
		if err := Validate(c.value, c.typ); err != nil {
			t.Errorf("Validate(%q, %q) unexpected error: %v", c.value, c.typ, err)
		}
	}
}

func TestValidate_Invalid(t *testing.T) {
	cases := []struct {
		value string
		typ   InputType
	}{
		{"not-an-email", TypeEmail},
		{"not a domain!", TypeDomain},
		{"999.999.999.999", TypeIP},
		{"a", TypeUsername}, // çok kısa
		{"not-a-url", TypeURL},
		{"not-a-hash", TypeHash},
		{"", TypeEmail}, // boş
	}

	for _, c := range cases {
		if err := Validate(c.value, c.typ); err == nil {
			t.Errorf("Validate(%q, %q) expected error, got nil", c.value, c.typ)
		}
	}
}

func TestAllTypes(t *testing.T) {
	types := AllTypes()
	if len(types) != 6 {
		t.Errorf("expected 6 types, got %d", len(types))
	}
}
