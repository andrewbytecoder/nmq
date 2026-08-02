package types

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
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
	CertFile FileOrContent `json:"certFile,omitempty" toml:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile  FileOrContent `json:"keyFile,omitempty" toml:"keyFile,omitempty" yaml:"keyFile,omitempty" loggable:"false"`
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

// SANType is the type of the Subject Alternative Name.
type SANType string

const (
	// SANDNSNameType specifies hostname-based SAN.
	SANDNSNameType SANType = "DNSName"

	// SANURIType specifies URI-based SAN, e.g. SPIFFE id.
	SANURIType SANType = "URI"
)

// +k8s:deepcopy-gen=true

// SAN represents a Subject Alternative Name.
type SAN struct {
	Type  SANType `json:"type,omitempty" toml:"type,omitempty" yaml:"type,omitempty"`
	Value string  `json:"value,omitempty" toml:"value,omitempty" yaml:"value,omitempty"`
}

// VerifyPeerCertificate verifies the chain certificates and their URI.
func VerifyPeerCertificate(sans []SAN, rootCAs *x509.CertPool, rawCerts [][]byte) error {
	// TODO: Refactor to avoid useless verifyChain (ex: when insecureskipverify is false)
	cert, err := verifyChain(rootCAs, rawCerts)
	if err != nil {
		return fmt.Errorf("verifying chain: %w", err)
	}

	for _, san := range sans {
		switch san.Type {
		case SANURIType:
			if slices.ContainsFunc(cert.URIs, func(uri *url.URL) bool {
				return strings.EqualFold(san.Value, uri.String())
			}) {
				return nil
			}

		case SANDNSNameType:
			if err := cert.VerifyHostname(san.Value); err == nil {
				return nil
			}
		}
	}

	return errors.New("no matching SAN in peer certificate")
}

// verifyChain performs standard TLS verification without enforcing remote hostname matching.
func verifyChain(rootCAs *x509.CertPool, rawCerts [][]byte) (*x509.Certificate, error) {
	// Fetch leaf and intermediates. This is based on code form tls handshake.
	if len(rawCerts) < 1 {
		return nil, errors.New("tls: no certificates from peer")
	}

	certs := make([]*x509.Certificate, len(rawCerts))
	for i, asn1Data := range rawCerts {
		cert, err := x509.ParseCertificate(asn1Data)
		if err != nil {
			return nil, fmt.Errorf("tls: failed to parse certificate from peer: %w", err)
		}

		certs[i] = cert
	}

	opts := x509.VerifyOptions{
		Roots:         rootCAs,
		Intermediates: x509.NewCertPool(),
	}

	// All but the first cert are intermediates
	for _, cert := range certs[1:] {
		opts.Intermediates.AddCert(cert)
	}

	_, err := certs[0].Verify(opts)
	if err != nil {
		return nil, err
	}

	return certs[0], nil
}
