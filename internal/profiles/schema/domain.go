package schema

import "fmt"

// Domain is the search profile kind.
type Domain string

const (
	DomainSoftware     Domain = "software"
	DomainArchitecture Domain = "architecture"
)

func (d Domain) String() string { return string(d) }

func (d Domain) Equals(s string) bool { return string(d) == s }

func (d Domain) Pointer() *Domain { return &d }

func (d Domain) FromValue(s string) (Domain, error) {
	switch Domain(s) {
	case DomainSoftware, DomainArchitecture:
		return Domain(s), nil
	default:
		return "", fmt.Errorf("unknown Domain %q: valid values are %v", s, ValuesDomain())
	}
}

func ValuesDomain() []Domain {
	return []Domain{DomainSoftware, DomainArchitecture}
}

func FromStringDomain(s string) (Domain, error) {
	var z Domain
	return z.FromValue(s)
}
