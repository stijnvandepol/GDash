package api

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type forwardedHTTPSKey struct{}

// behindProxies trusts the X-Forwarded-For and X-Forwarded-Proto headers of
// requests that come from one of trusted, such as a reverse proxy that
// serves the web interface over HTTPS. The client address then becomes the
// address the proxy saw, for the login limits and the audit log, and
// cookies get the Secure flag when the proxy received HTTPS. Headers from
// anyone else are ignored: a client could write them itself.
func behindProxies(trusted []netip.Prefix, next http.Handler) http.Handler {
	if len(trusted) == 0 {
		return next
	}
	isTrusted := func(addr netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(addr.Unmap()) {
				return true
			}
		}
		return false
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := netip.ParseAddr(clientIP(r))
		if err != nil || !isTrusted(peer) {
			next.ServeHTTP(w, r)
			return
		}
		r = r.Clone(r.Context())
		if client, ok := forwardedClient(r.Header.Values("X-Forwarded-For"), isTrusted); ok {
			r.RemoteAddr = net.JoinHostPort(client.String(), "0")
		}
		if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
			r = r.WithContext(context.WithValue(r.Context(), forwardedHTTPSKey{}, true))
		}
		next.ServeHTTP(w, r)
	})
}

// forwardedClient returns the client address from X-Forwarded-For: the
// rightmost address that is not one of the trusted proxies. Addresses
// further left were written by the client and can be anything.
func forwardedClient(headers []string, isTrusted func(netip.Addr) bool) (netip.Addr, bool) {
	var hops []string
	for _, h := range headers {
		hops = append(hops, strings.Split(h, ",")...)
	}
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return netip.Addr{}, false
		}
		if !isTrusted(addr) {
			return addr.Unmap(), true
		}
	}
	return netip.Addr{}, false
}

// isHTTPS reports whether the browser reached Nookboard over HTTPS, directly
// or through a trusted proxy, so cookies may be marked Secure.
func isHTTPS(r *http.Request) bool {
	forwarded, _ := r.Context().Value(forwardedHTTPSKey{}).(bool)
	return r.TLS != nil || forwarded
}
