package custom

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/stijnvandepol/gdash/internal/validation"
)

// Authentication kinds.
const (
	AuthNone   = "none"
	AuthBearer = "bearer"
	AuthHeader = "header"
	AuthQuery  = "query"
	AuthBasic  = "basic"
)

// Auth is how a request to an address an admin typed in authenticates:
// one of the kinds above, the header, query parameter or user name, and
// the token, API key or password. Custom sources and cameras share it.
type Auth struct {
	Kind   string
	Name   string
	Secret string
}

// CheckAuth validates in for a request to address and returns the
// authentication to store. An empty secret keeps old's, but only for the
// same kind: a bearer token must not become a query parameter. Problems
// are added to errs as auth.kind, auth.name and auth.secret; messages
// never contain a secret.
func CheckAuth(errs *validation.Collector, in, old Auth, address string) Auth {
	secret := in.Secret
	if secret == "" && old.Kind == in.Kind {
		secret = old.Secret
	}
	switch in.Kind {
	case AuthNone:
		return Auth{Kind: AuthNone}
	case AuthBearer:
		if !plainSecret(secret, false) {
			errs.Add("auth.secret", "paste the token, without spaces")
		}
	case AuthHeader:
		if !headerName.MatchString(in.Name) || reservedHeaders[strings.ToLower(in.Name)] && !strings.EqualFold(in.Name, "authorization") {
			errs.Add("auth.name", "use a header name such as X-API-Key")
		}
		if !plainSecret(secret, true) {
			errs.Add("auth.secret", "paste the API key")
		}
	case AuthQuery:
		if !paramName.MatchString(in.Name) {
			errs.Add("auth.name", "use a parameter name such as appid or api_key")
		} else if u, err := url.Parse(address); address != "" && err == nil && u.Query().Has(in.Name) {
			errs.Add("auth.name", "the address already has this parameter; remove it from the address")
		}
		if !plainSecret(secret, true) {
			errs.Add("auth.secret", "paste the API key")
		}
	case AuthBasic:
		if in.Name == "" || len(in.Name) > 128 || strings.Contains(in.Name, ":") || validation.HasControlCharacter(in.Name) {
			errs.Add("auth.name", "use a user name of at most 128 characters without a colon")
		}
		if !plainSecret(secret, true) {
			errs.Add("auth.secret", "enter the password")
		}
	default:
		errs.Add("auth.kind", "choose none, bearer, header, query or basic")
		return Auth{}
	}
	name := in.Name
	if in.Kind == AuthBearer {
		name = ""
	}
	return Auth{Kind: in.Kind, Name: name, Secret: secret}
}

// plainSecret reports whether s can be sent in a header or query: not
// empty, not too long and without control characters (a line break would
// add a header). Tokens also may not hold spaces.
func plainSecret(s string, spaces bool) bool {
	return s != "" && len(s) <= maxSecretBytes && !validation.HasControlCharacter(s) &&
		(spaces || !strings.ContainsAny(s, " "))
}

// Apply puts the authentication on req. The key of query authentication
// is added here and never stored in the address.
func (a Auth) Apply(req *http.Request) {
	switch a.Kind {
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+a.Secret)
	case AuthHeader:
		req.Header.Set(a.Name, a.Secret)
	case AuthBasic:
		req.SetBasicAuth(a.Name, a.Secret)
	case AuthQuery:
		q := req.URL.Query()
		q.Set(a.Name, a.Secret)
		req.URL.RawQuery = q.Encode()
	}
}
