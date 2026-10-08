package pairing

import (
	"crypto/rand"
	"fmt"
	"strings"
)

func randomCode() (string, error) {
	b := make([]byte, codeLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate pairing code: %w", err)
	}
	code := make([]byte, codeLength)
	for i, v := range b {
		// 256 is not a multiple of 31, so this is very slightly biased; at
		// 31^8 possible codes with a 10-minute lifetime that does not matter.
		code[i] = codeAlphabet[int(v)%len(codeAlphabet)]
	}
	return string(code), nil
}

// formatCode shows a code as two groups of four, easier to read off a TV.
func formatCode(code string) string { return code[:4] + "-" + code[4:] }

func normalizeCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, strings.ToUpper(code))
}
