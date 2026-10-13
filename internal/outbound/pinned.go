package outbound

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Fingerprint is the SHA-256 of a server's certificate, in DER form.
type Fingerprint [sha256.Size]byte

// ErrCertificateChanged is returned when a server presents another
// certificate than the one an admin chose to trust.
var ErrCertificateChanged = errors.New("the server presents another certificate than the one that was accepted")

// CertificateFingerprint connects to the TLS server at address (host:port),
// through the same address guard as NewClient, and returns the fingerprint
// of the certificate it presents. It does not check the certificate: it is
// how an admin who chose to accept a home server's self-signed certificate
// learns which one to pin.
func CertificateFingerprint(ctx context.Context, address string, timeout time.Duration) (Fingerprint, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: dialTimeout, Control: control},
		// Nothing is sent over this connection: it only reads the
		// certificate, which the caller then pins.
		Config: &tls.Config{InsecureSkipVerify: true},
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return Fingerprint{}, fmt.Errorf("read the certificate of %s: %w", address, err)
	}
	defer conn.Close()
	certs := conn.(*tls.Conn).ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return Fingerprint{}, fmt.Errorf("%s presents no certificate", address)
	}
	return sha256.Sum256(certs[0].Raw), nil
}

// NewPinnedClient returns a client like NewClient that, for https, accepts
// exactly the server certificate with fingerprint pin instead of one that a
// public authority signed: a NAS at home usually has a self-signed one.
// Any other certificate fails with ErrCertificateChanged.
func NewPinnedClient(timeout time.Duration, pin Fingerprint) *http.Client {
	client := NewClient(timeout)
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{
		// The default check is replaced by the pin below, which is
		// stricter: only this one certificate is accepted.
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if len(state.PeerCertificates) == 0 || sha256.Sum256(state.PeerCertificates[0].Raw) != pin {
				return ErrCertificateChanged
			}
			return nil
		},
	}
	return client
}
