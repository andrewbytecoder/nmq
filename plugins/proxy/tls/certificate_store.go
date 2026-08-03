package tls

import (
	"crypto/tls"

	"github.com/andrewbytecoder/nmq/pkg/safe"
	"github.com/patrickmn/go-cache"
)

// CertificateData holds runtime data for runtime TLS certificate handling.
type CertificateData struct {
	Hash        string
	Certificate *tls.Certificate
}

// CertificateStore store for dynamic certificates.
type CertificateStore struct {
	DynamicCerts       *safe.Safe
	DefaultCertificate *CertificateData
	CertCache          *cache.Cache

	ocspStapler *ocspStapler
}
