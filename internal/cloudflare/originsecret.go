package cloudflare

import (
	"fmt"
	"io/ioutil"
	"strings"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/validation"
)

type multierror struct {
	causes []error
	detail string
}

// Error collapses the MultiError into a string
func (e *multierror) Error() string {
	if e.detail == "" {
		var a = make([]string, len(e.causes))
		for i := 0; i < len(e.causes); i++ {
			a[i] = e.causes[i].Error()
		}
		e.detail = strings.Join(a, ", ")
	}
	return e.detail
}

// ParseOriginSecrets parses a origin certificate mapping
func ParseOriginSecrets(b []byte) (*OriginSecrets, error) {
	var oc OriginSecrets
	if err := yaml.UnmarshalStrict(b, &oc); err != nil {
		return nil, err
	}
	if errs := oc.Validate(); len(errs) > 0 {
		return nil, &multierror{
			causes: oc.Validate(),
		}
	}
	return &oc, nil
}

// ParseOriginSecretsFile parses a origin certificate mapping file
func ParseOriginSecretsFile(file string) (oc *OriginSecrets, err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return
	}
	return ParseOriginSecrets(b)
}

// OriginSecrets is a mapping of origins to secrets
type OriginSecrets struct {
	Groups []OriginSecretGroup `yaml:"groups"`
}

// Validate the OriginCerts content
func (oc *OriginSecrets) Validate() []error {
	var errs []error
	for i, group := range oc.Groups {
		if es := group.Validate(); len(es) > 0 {
			for _, e := range es {
				errs = append(errs, fmt.Errorf("group at index %d, %s", i, e.Error()))
			}
		}
	}
	return errs
}

// OriginSecretGroup groups a set of origins to a secret
type OriginSecretGroup struct {
	Hosts  []string     `yaml:"hosts"`
	Secret OriginSecret `yaml:"secret"`
}

// Validate the OriginSecretGroup content
func (ocg *OriginSecretGroup) Validate() []error {
	var errs []error
	if len(ocg.Hosts) == 0 {
		errs = append(errs, fmt.Errorf("hosts %s", validation.EmptyError()))
	} else {
		for i, host := range ocg.Hosts {
			if len(host) == 0 {
				errs = append(errs, fmt.Errorf("host at index %d %s", i, validation.EmptyError()))
			} else if strings.Contains(host, "*") {
				if host != "*" {
					for _, msg := range validation.IsWildcardDNS1123Subdomain(host) {
						errs = append(errs, fmt.Errorf("host %q at index %d %s", host, i, msg))
					}
				}
			} else {
				for _, msg := range validation.IsDNS1123Subdomain(host) {
					errs = append(errs, fmt.Errorf("host %q at index %d %s", host, i, msg))
				}
			}
		}
	}
	for _, e := range ocg.Secret.Validate() {
		errs = append(errs, fmt.Errorf("secret %s", e.Error()))
	}
	return errs
}

// OriginSecret defines a secret
type OriginSecret struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// Validate the OriginSecret content
func (os *OriginSecret) Validate() []error {
	var errs []error
	if len(os.Name) == 0 {
		errs = append(errs, fmt.Errorf("name %s", validation.EmptyError()))
	} else if strings.Contains(os.Name, "/") {
		errs = append(errs, fmt.Errorf("name %q must not contain '/'", os.Name))
	} else {
		for _, msg := range validation.IsQualifiedName(os.Name) {
			errs = append(errs, fmt.Errorf("name %q %s", os.Name, msg))
		}
	}
	if len(os.Namespace) == 0 {
		errs = append(errs, fmt.Errorf("namespace %s", validation.EmptyError()))
	} else {
		for _, msg := range validation.IsDNS1123Subdomain(os.Namespace) {
			errs = append(errs, fmt.Errorf("namespace %q %s", os.Namespace, msg))
		}
	}
	return errs
}
