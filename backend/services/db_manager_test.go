package services

import (
	"strings"
	"testing"
)

func TestToPostgresName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"backroom", "backroom"},
		{"backroom-api", "backroom_api"},
		{"My-Project-123", "my_project_123"},
	}

	for _, tt := range tests {
		got := ToPostgresName(tt.input)
		if got != tt.want {
			t.Errorf("ToPostgresName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateSecurePassword(t *testing.T) {
	p1 := GenerateSecurePassword(32)
	p2 := GenerateSecurePassword(32)

	if len(p1) < 32 {
		t.Errorf("password length %d is less than 32", len(p1))
	}
	if p1 == p2 {
		t.Errorf("two random passwords should not be equal: %s == %s", p1, p2)
	}
}

func TestPostgresIdentifierValidation(t *testing.T) {
	invalidNames := []string{
		"backroom; DROP TABLE users;",
		"back-room",             // contains dash, must be converted with ToPostgresName first
		"ab",                    // too short
		strings.Repeat("a", 55), // too long
		"123!@#",
	}

	for _, name := range invalidNames {
		if validPostgresIdentifier.MatchString(name) {
			t.Errorf("expected identifier %q to be invalid, but regex matched", name)
		}
	}

	validNames := []string{
		"backroom_db",
		"backroom_user",
		"app_123_db",
	}

	for _, name := range validNames {
		if !validPostgresIdentifier.MatchString(name) {
			t.Errorf("expected identifier %q to be valid, but regex failed", name)
		}
	}
}
