package env

import (
	"os"
	"strings"
)

// Getenv returns the value of the environment variable key,
// or def if the variable is unset or blank.
func Getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
