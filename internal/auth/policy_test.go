package auth

import (
	"strings"
	"testing"
)

func TestPasswordProblem(t *testing.T) {
	tests := []struct {
		name, password, want string
	}{
		{"a long passphrase", "correct horse battery", ""},
		{"too short", "short pass1", "at least 12"},
		{"twelve characters", "kitchen-tv-9", ""},
		{"long in another script counts characters", strings.Repeat("жук ", 32), ""},
		{"too long", strings.Repeat("a1", 65), "at most 128"},
		{"contains the user name", "my stijn password", "user name"},
		{"well known", "Password1234", "well known"},
		{"well known with spaces", "correct horse battery staple", "well known"},
		{"one repeated character", "aaaaaaaaaaaa", "row or repeat"},
		{"digits in a row", "123456789012", "row or repeat"},
		{"letters downwards", "zyxwvutsrqpo", "row or repeat"},
		{"invalid UTF-8", "abcdefghijk\xff", "valid text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := passwordProblem(tt.password, "Stijn")
			if (tt.want == "") != (got == "") || !strings.Contains(got, tt.want) {
				t.Fatalf("passwordProblem(%q) = %q, want %q", tt.password, got, tt.want)
			}
		})
	}
}

func TestShortUserNamesMayAppearInPasswords(t *testing.T) {
	if got := passwordProblem("a long passphrase", "ab"); got != "" {
		t.Fatalf("a two-letter user name blocked the password: %q", got)
	}
}

func TestCommonPasswordListIsLoaded(t *testing.T) {
	if len(commonPasswords) < 50 || commonPasswords["# well-known passwords of at least 12 characters, lower case, without"] {
		t.Fatalf("common password list has %d entries, or kept its comments", len(commonPasswords))
	}
	for p := range commonPasswords {
		if len([]rune(p)) < minPasswordLength || p != strings.ToLower(p) || strings.Contains(p, " ") {
			t.Errorf("list entry %q is shorter than the minimum, not lower case or has a space", p)
		}
	}
}
