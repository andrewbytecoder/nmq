package tls

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/andrewbytecoder/nmq/pkg/types"
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

const certificateHeader = "-----BEGIN CERTIFICATE-----\n"

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
