package auth

import (
	_ "embed"
	"fmt"
	"strings"
	"unicode/utf8"
)

// The password rules follow NIST SP 800-63B: a minimum length, a generous
// maximum, a check against well-known passwords, and no rules about which
// kinds of characters to use.
const (
	minPasswordLength = 12
	// maxPasswordLength is counted in characters, so a long passphrase in
	// any script fits.
	maxPasswordLength = 128
	// maxPasswordBytes caps what reaches argon2 at all, also at login: long
	// inputs make it slow for no gain. 128 characters of up to 4 bytes fit.
	maxPasswordBytes = 4 * maxPasswordLength
	// minNameInPassword is the shortest user name that a password may not
	// contain; a one-letter name would forbid half of all words.
	minNameInPassword = 3
)

// commonPasswordList holds passwords of at least minPasswordLength
// characters that appear high in lists of leaked passwords, lower case,
// one per line. It is no replacement for a breach corpus, which would not
// fit a small self-hosted program, but it stops the obvious ones.
//
//go:embed common_passwords.txt
var commonPasswordList string

var commonPasswords = func() map[string]bool {
	set := map[string]bool{}
	for line := range strings.Lines(commonPasswordList) {
		if p := strings.TrimSpace(line); p != "" && !strings.HasPrefix(p, "#") {
			set[p] = true
		}
	}
	return set
}()

// passwordProblem returns why password is not acceptable as a new password
// for username, or "".
func passwordProblem(password, username string) string {
	length := utf8.RuneCountInString(password)
	lower := strings.ToLower(password)
	name := strings.ToLower(username)
	switch {
	case !utf8.ValidString(password):
		return "use valid text"
	case length < minPasswordLength:
		return fmt.Sprintf("use at least %d characters", minPasswordLength)
	case length > maxPasswordLength:
		return fmt.Sprintf("use at most %d characters", maxPasswordLength)
	case len(name) >= minNameInPassword && strings.Contains(lower, name):
		return "do not use your user name in the password"
	case commonPasswords[lower] || commonPasswords[strings.ReplaceAll(lower, " ", "")]:
		return "this password is well known; choose another"
	case isRunOrRepeat(lower):
		return "use something less predictable than a row or repeat of characters"
	}
	return ""
}

// isRunOrRepeat reports a password that is one character repeated, or a
// run such as "123456789012" or "abcdefghijkl" upwards or downwards.
func isRunOrRepeat(s string) bool {
	runes := []rune(s)
	same, up, down := true, true, true
	for i := 1; i < len(runes); i++ {
		step := runes[i] - runes[i-1]
		same = same && step == 0
		// Digits wrap from 9 to 0, as on a keyboard row.
		up = up && (step == 1 || runes[i-1] == '9' && runes[i] == '0')
		down = down && (step == -1 || runes[i-1] == '0' && runes[i] == '9')
	}
	return same || up || down
}
