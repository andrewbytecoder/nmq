package tls

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/andrewbytecoder/nmq/pkg/types"
)

var (
	// MinVersion Map of allowed TLS minimum versions.
	MinVersion = map[string]uint16{
		`VersionTLS10`: tls.VersionTLS10,
		`VersionTLS11`: tls.VersionTLS11,
		`VersionTLS12`: tls.VersionTLS12,
		`VersionTLS13`: tls.VersionTLS13,
	}

	// MaxVersion Map of allowed TLS maximum versions.
	MaxVersion = map[string]uint16{
		`VersionTLS10`: tls.VersionTLS10,
		`VersionTLS11`: tls.VersionTLS11,
		`VersionTLS12`: tls.VersionTLS12,
		`VersionTLS13`: tls.VersionTLS13,
	}

	// CurveIDs is a Map of TLS elliptic curves from crypto/tls
	// Available CurveIDs defined at https://godoc.org/crypto/tls#CurveID,
	// also allowing rfc names defined at https://tools.ietf.org/html/rfc8446#section-4.2.7
	CurveIDs = map[string]tls.CurveID{
		`secp256r1`:      tls.CurveP256,
		`CurveP256`:      tls.CurveP256,
		`secp384r1`:      tls.CurveP384,
		`CurveP384`:      tls.CurveP384,
		`secp521r1`:      tls.CurveP521,
		`CurveP521`:      tls.CurveP521,
		`x25519`:         tls.X25519,
		`X25519`:         tls.X25519,
		`x25519mlkem768`: tls.X25519MLKEM768,
		`X25519MLKEM768`: tls.X25519MLKEM768,
		// Post-quantum hybrid key exchanges enabled by default since Go 1.26.
		`secp256r1mlkem768`:  tls.SecP256r1MLKEM768,
		`SecP256r1MLKEM768`:  tls.SecP256r1MLKEM768,
		`secp384r1mlkem1024`: tls.SecP384r1MLKEM1024,
		`SecP384r1MLKEM1024`: tls.SecP384r1MLKEM1024,
	}
)

// Certificates defines traefik certificates type
// Certs and Keys could be either a file path, or the file content itself.
type Certificates []Certificate

// GetCertificates retrieves the certificates as slice of tls.Certificate.
func (c Certificates) GetCertificates() []tls.Certificate {
	var certs []tls.Certificate

	for _, certificate := range c {
		cert, err := certificate.GetCertificate()
		if err != nil {
			//log.Debug().Err(err).Msg("Error while getting certificate")
			continue
		}

		certs = append(certs, cert)
	}

	return certs
}

// Certificate holds a SSL cert/key pair
// Certs and Key could be either a file path, or the file content itself.
type Certificate struct {
	CertFile types.FileOrContent `json:"certFile,omitempty" toml:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile  types.FileOrContent `json:"keyFile,omitempty" toml:"keyFile,omitempty" yaml:"keyFile,omitempty" loggable:"false"`
}

// GetCertificate returns a tls.Certificate matching the configured CertFile and KeyFile.
func (c *Certificate) GetCertificate() (tls.Certificate, error) {
	certContent, err := c.CertFile.Read()
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("unable to read CertFile: %w", err)
	}

	keyContent, err := c.KeyFile.Read()
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("unable to read KeyFile: %w", err)
	}

	cert, err := tls.X509KeyPair(certContent, keyContent)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("unable to parse TLS certificate: %w", err)
	}

	return cert, nil
}

// GetCertificateFromBytes returns a tls.Certificate matching the configured CertFile and KeyFile.
// It assumes that the configured CertFile and KeyFile are of byte type.
func (c *Certificate) GetCertificateFromBytes() (tls.Certificate, error) {
	cert, err := tls.X509KeyPair([]byte(c.CertFile), []byte(c.KeyFile))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("unable to parse TLS certificate: %w", err)
	}

	return cert, nil
}

// GetTruncatedCertificateName truncates the certificate name.
func (c *Certificate) GetTruncatedCertificateName() string {
	certName := c.CertFile.String()

	// Truncate certificate information only if it's a well formed certificate content with more than 50 characters
	if !c.CertFile.IsPath() && strings.HasPrefix(certName, certificateHeader) && len(certName) > len(certificateHeader)+50 {
		certName = strings.TrimPrefix(c.CertFile.String(), certificateHeader)[:50]
	}

	return certName
}
