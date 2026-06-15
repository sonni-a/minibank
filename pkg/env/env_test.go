package env

import "testing"

func TestGetenv(t *testing.T) {
	key := "MINIBANK_TEST_GETENV"

	t.Setenv(key, "")
	if got := Getenv(key, "default"); got != "default" {
		t.Fatalf("Getenv unset = %q, want default", got)
	}

	t.Setenv(key, "  value  ")
	if got := Getenv(key, "default"); got != "value" {
		t.Fatalf("Getenv trimmed = %q, want value", got)
	}

	t.Setenv(key, "   ")
	if got := Getenv(key, "default"); got != "default" {
		t.Fatalf("Getenv blank = %q, want default", got)
	}

	t.Setenv(key, "explicit")
	if got := Getenv(key, "default"); got != "explicit" {
		t.Fatalf("Getenv set = %q, want explicit", got)
	}
}
