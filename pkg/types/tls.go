package types

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"
)

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

// +k8s:deepcopy-gen=true

// ClientTLS holds TLS specific configurations as client
// CA, Cert and Key can be either path or file contents.
type ClientTLS struct {
	CA                 string `description:"TLS CA" json:"ca,omitempty" toml:"ca,omitempty" yaml:"ca,omitempty"`
	Cert               string `description:"TLS cert" json:"cert,omitempty" toml:"cert,omitempty" yaml:"cert,omitempty"`
	Key                string `description:"TLS key" json:"key,omitempty" toml:"key,omitempty" yaml:"key,omitempty" loggable:"false"`
	InsecureSkipVerify bool   `description:"TLS insecure skip verify" json:"insecureSkipVerify,omitempty" toml:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty" export:"true"`
}

// CreateTLSConfig creates a TLS config from ClientTLS structures.
func (c *ClientTLS) CreateTLSConfig(ctx context.Context) (*tls.Config, error) {
	if c == nil {
		return nil, nil
	}

	// Not initialized, to rely on system bundle.
	var caPool *x509.CertPool

	if c.CA != "" {
		var ca []byte
		if _, errCA := os.Stat(c.CA); errCA == nil {
			var err error
			ca, err = os.ReadFile(c.CA)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA. %w", err)
			}
		} else {
			ca = []byte(c.CA)
		}

		caPool = x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(ca) {
			return nil, errors.New("failed to parse CA")
		}
	}

	hasCert := len(c.Cert) > 0
	hasKey := len(c.Key) > 0

	if hasCert != hasKey {
		return nil, errors.New("both TLS cert and key must be defined")
	}

	if !hasCert || !hasKey {
		return &tls.Config{
			RootCAs:            caPool,
			InsecureSkipVerify: c.InsecureSkipVerify,
		}, nil
	}

	cert, err := loadKeyPair(c.Cert, c.Key)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caPool,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}, nil
}

func loadKeyPair(cert, key string) (tls.Certificate, error) {
	keyPair, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err == nil {
		return keyPair, nil
	}

	_, err = os.Stat(cert)
	if err != nil {
		return tls.Certificate{}, errors.New("cert file does not exist")
	}

	_, err = os.Stat(key)
	if err != nil {
		return tls.Certificate{}, errors.New("key file does not exist")
	}

	keyPair, err = tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return tls.Certificate{}, err
	}

	return keyPair, nil
}
