package validate

import "regexp"

var emailRe = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func Email(s string) bool {
	return emailRe.MatchString(s)
}

func Password(s string) bool {
	return len(s) >= 8
}
