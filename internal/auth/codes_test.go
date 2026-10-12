package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stijnvandepol/gdash/internal/totp"
)

// Codes are pasted from password managers and documents, with tabs and
// non-breaking spaces as often as plain ones.
func TestCodesAcceptAnyWhiteSpace(t *testing.T) {
	ctx := context.Background()
	s, clock := newTestService(t)
	secret, recovery := enableTwoStep(t, s, setUp(t, s))
	for i, space := range []string{"\t", " ", " ", " \t"} {
		clock.t = clock.t.Add(totp.Period * time.Second)
		code := totp.Code(secret, totp.Step(clock.t))
		pending := passwordStep(t, s, homeIP)
		if _, _, _, err := s.FinishLogin(ctx, pending, space+code[:3]+space+code[3:]+space, homeIP); err != nil {
			t.Fatalf("app code with %q: %v", space, err)
		}
		pending = passwordStep(t, s, homeIP)
		pasted := strings.ReplaceAll(strings.ToUpper(recovery[i]), "-", space)
		if _, _, use, err := s.FinishLogin(ctx, pending, pasted, homeIP); err != nil || !use.Recovery {
			t.Fatalf("recovery code with %q = %+v, %v", space, use, err)
		}
	}
}
