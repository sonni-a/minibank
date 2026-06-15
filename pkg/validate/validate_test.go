package validate

import "testing"

func TestEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"user@example.com", true},
		{"user.name+tag@domain.co.uk", true},
		{"invalid", false},
		{"", false},
		{"user@domain", false},
		{"@example.com", false},
		{"user@", false},
	}

	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			if got := Email(tt.email); got != tt.want {
				t.Fatalf("Email(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestPassword(t *testing.T) {
	tests := []struct {
		password string
		want     bool
	}{
		{"1234567", false},
		{"12345678", true},
		{"longpassword", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			if got := Password(tt.password); got != tt.want {
				t.Fatalf("Password(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}
