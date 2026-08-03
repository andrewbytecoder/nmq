package tls

import "github.com/andrewbytecoder/nmq/pkg/types"

// +k8s:deepcopy-gen=true

// Store holds the options for a given Store.
type Store struct {
	DefaultCertificate   *Certificate   `json:"defaultCertificate,omitempty" toml:"defaultCertificate,omitempty" yaml:"defaultCertificate,omitempty" label:"-" export:"true"`
	DefaultGeneratedCert *GeneratedCert `json:"defaultGeneratedCert,omitempty" toml:"defaultGeneratedCert,omitempty" yaml:"defaultGeneratedCert,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// GeneratedCert defines the default generated certificate configuration.
type GeneratedCert struct {
	// Resolver is the name of the resolver that will be used to issue the DefaultCertificate.
	Resolver string `json:"resolver,omitempty" toml:"resolver,omitempty" yaml:"resolver,omitempty" export:"true"`
	// Domain is the domain definition for the DefaultCertificate.
	Domain *types.Domain `json:"domain,omitempty" toml:"domain,omitempty" yaml:"domain,omitempty" export:"true"`
}

// +k8s:deepcopy-gen=true

// CertAndStores allows mapping a TLS certificate to a list of entry points.
type CertAndStores struct {
	Certificate `yaml:",inline" export:"true"`

	Stores []string `json:"stores,omitempty" toml:"stores,omitempty" yaml:"stores,omitempty" export:"true"`
}
