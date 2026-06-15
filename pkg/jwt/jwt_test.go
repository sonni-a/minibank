package jwt

import (
	"strings"
	"testing"
)

func setJWTEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-access-secret")
	t.Setenv("JWT_REFRESH_SECRET", "test-refresh-secret")
}

func TestGenerateAndValidateJWT(t *testing.T) {
	setJWTEnv(t)

	token, err := GenerateJWT("user@example.com")
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	email, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("email = %q, want user@example.com", email)
	}
}

func TestGenerateJWT_MissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	_, err := GenerateJWT("user@example.com")
	if err == nil {
		t.Fatal("expected error when JWT_SECRET is not set")
	}
	if !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	setJWTEnv(t)

	_, err := ValidateJWT("not-a-valid-token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestGenerateAndValidateRefreshToken(t *testing.T) {
	setJWTEnv(t)

	token, err := GenerateRefreshToken("user@example.com")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty refresh token")
	}

	email, err := ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if email != "user@example.com" {
		t.Fatalf("email = %q, want user@example.com", email)
	}
}

func TestValidateRefreshToken_WrongSecret(t *testing.T) {
	setJWTEnv(t)

	token, err := GenerateRefreshToken("user@example.com")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	t.Setenv("JWT_REFRESH_SECRET", "other-secret")
	_, err = ValidateRefreshToken(token)
	if err == nil {
		t.Fatal("expected error when refresh secret does not match")
	}
}
