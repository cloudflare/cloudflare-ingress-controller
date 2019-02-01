package cloudflare

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type unitfile struct {
	name string
	data []byte
	mode os.FileMode
}

func TestOriginSecrets(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  *OriginSecrets
		err []error
	}{
		"obj-empty": {
			in:  &OriginSecrets{},
			err: nil,
		},
		"obj-empty-in-group": {
			in: &OriginSecrets{
				Groups: []OriginSecretGroup{
					{
						Hosts: []string{},
					},
				},
			},
			err: []error{
				fmt.Errorf("group at index 0, hosts must be non-empty"),
				fmt.Errorf("group at index 0, secret name must be non-empty"),
				fmt.Errorf("group at index 0, secret namespace must be non-empty"),
			},
		},
		"obj-bad-hosts-in-group": {
			in: &OriginSecrets{
				Groups: []OriginSecretGroup{
					{
						Hosts: []string{
							"",
							"*.*.test.com",
							"#@!.test.com",
						},
						Secret: OriginSecret{
							Name:      "test",
							Namespace: "test",
						},
					},
				},
			},
			err: []error{
				fmt.Errorf("group at index 0, host at index 0 must be non-empty"),
				fmt.Errorf(`group at index 0, host "*.*.test.com" at index 1 a wildcard DNS-1123 subdomain must start with '*.', followed by a valid DNS subdomain, which must consist of lower case alphanumeric characters, '-' or '.' and end with an alphanumeric character (e.g. '*.example.com', regex used for validation is '\*\.[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`),
				fmt.Errorf(`group at index 0, host "#@!.test.com" at index 2 a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`),
			},
		},
		"obj-bad-secret-name-in-group": {
			in: &OriginSecrets{
				Groups: []OriginSecretGroup{
					{
						Hosts: []string{
							"0.test.com",
						},
						Secret: OriginSecret{
							Name:      "",
							Namespace: "test",
						},
					},
					{
						Hosts: []string{
							"1.test.com",
						},
						Secret: OriginSecret{
							Name:      "unit/test",
							Namespace: "test",
						},
					},
					{
						Hosts: []string{
							"2.test.com",
						},
						Secret: OriginSecret{
							Name:      "@test@",
							Namespace: "test",
						},
					},
				},
			},
			err: []error{
				fmt.Errorf("group at index 0, secret name must be non-empty"),
				fmt.Errorf(`group at index 1, secret name "unit/test" must not contain '/'`),
				fmt.Errorf(`group at index 2, secret name "@test@" name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`),
			},
		},
		"obj-bad-secret-namespace-in-group": {
			in: &OriginSecrets{
				Groups: []OriginSecretGroup{
					{
						Hosts: []string{
							"0.test.com",
						},
						Secret: OriginSecret{
							Name:      "test",
							Namespace: "",
						},
					},
					{
						Hosts: []string{
							"1.test.com",
						},
						Secret: OriginSecret{
							Name:      "test",
							Namespace: "@test@",
						},
					},
				},
			},
			err: []error{
				fmt.Errorf("group at index 0, secret namespace must be non-empty"),
				fmt.Errorf(`group at index 1, secret namespace "@test@" a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'example.com', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`),
			},
		},
		"obj-okay": {
			in: &OriginSecrets{
				Groups: []OriginSecretGroup{
					{
						Hosts: []string{
							"abc.test.com",
							"xyz.test.com",
							"*.test.com",
						},
						Secret: OriginSecret{
							Name:      "test",
							Namespace: "test",
						},
					},
				},
			},
			err: nil,
		},
	} {
		err := test.in.Validate()
		assert.Equalf(t, test.err, err, "test '%s' err mismatch", name)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  string
		out *OriginSecrets
		err error
	}{
		"obj-fail-unmarshal": {
			in:  "@",
			out: nil,
			err: fmt.Errorf(`yaml: found character that cannot start any token`),
		},
		"obj-empty": {
			in:  "",
			out: &OriginSecrets{},
			err: nil,
		},
		"obj-parse-error": {
			in:  errorCerts,
			out: nil,
			err: fmt.Errorf(`group at index 1, host "*.*.test.com" at index 0 a wildcard DNS-1123 subdomain must start with '*.', followed by a valid DNS subdomain, which must consist of lower case alphanumeric characters, '-' or '.' and end with an alphanumeric character (e.g. '*.example.com', regex used for validation is '\*\.[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`),
		},
		"obj-parse-okay": {
			in: okayCerts,
			out: &OriginSecrets{
				Groups: []OriginSecretGroup{
					{
						Hosts: []string{
							"abc.test.com",
						},
						Secret: OriginSecret{
							Name:      "test-a",
							Namespace: "test-a",
						},
					},
					{
						Hosts: []string{
							"xyz.test.com",
						},
						Secret: OriginSecret{
							Name:      "test-b",
							Namespace: "test-b",
						},
					},
					{
						Hosts: []string{
							"*.test.com",
						},
						Secret: OriginSecret{
							Name:      "test-c",
							Namespace: "test-c",
						},
					},
				},
			},
			err: nil,
		},
	} {
		out, err := ParseOriginSecrets([]byte(test.in))
		if err != nil {
			err = fmt.Errorf("%s", err.Error())
		}
		assert.Equalf(t, test.out, out, "test '%s' val mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' err mismatch", name)
	}
}

func TestParseFile(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		file unitfile
		mode os.FileMode
		out  *OriginSecrets
		err  error
	}{
		"obj-no-access": {
			file: unitfile{name: "test.yaml", data: []byte(""), mode: 0644},
			mode: 0000,
			out:  nil,
			err:  fmt.Errorf(`open test.yaml: permission denied`),
		},
		"obj-parse-empty": {
			file: unitfile{name: "test.yaml", data: []byte(""), mode: 0644},
			mode: 0700,
			out:  &OriginSecrets{},
			err:  nil,
		},
		"obj-parse-error": {
			file: unitfile{name: "test.yaml", data: []byte(errorCerts), mode: 0644},
			mode: 0700,
			out:  nil,
			err:  fmt.Errorf(`group at index 1, host "*.*.test.com" at index 0 a wildcard DNS-1123 subdomain must start with '*.', followed by a valid DNS subdomain, which must consist of lower case alphanumeric characters, '-' or '.' and end with an alphanumeric character (e.g. '*.example.com', regex used for validation is '\*\.[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*')`),
		},
		"obj-parse-okay": {
			file: unitfile{name: "test.yaml", data: []byte(okayCerts), mode: 0644},
			mode: 0700,
			out: &OriginSecrets{
				Groups: []OriginSecretGroup{
					{
						Hosts: []string{
							"abc.test.com",
						},
						Secret: OriginSecret{
							Name:      "test-a",
							Namespace: "test-a",
						},
					},
					{
						Hosts: []string{
							"xyz.test.com",
						},
						Secret: OriginSecret{
							Name:      "test-b",
							Namespace: "test-b",
						},
					},
					{
						Hosts: []string{
							"*.test.com",
						},
						Secret: OriginSecret{
							Name:      "test-c",
							Namespace: "test-c",
						},
					},
				},
			},
			err: nil,
		},
	} {
		rootdir, err := ioutil.TempDir("", "root-")
		assert.NoError(t, err, "must not error creating rootdir")
		defer os.RemoveAll(rootdir)

		filepath := filepath.Join(rootdir, test.file.name)
		ioutil.WriteFile(filepath, test.file.data, 0644)
		os.Chmod(filepath, test.file.mode)
		os.Chmod(rootdir, test.mode)

		out, err := ParseOriginSecretsFile(filepath)
		if err != nil {
			s := strings.Replace(err.Error(), filepath, test.file.name, -1)
			err = fmt.Errorf("%s", s)
		}
		assert.Equalf(t, test.out, out, "test '%s' val mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' err mismatch", name)
	}
}

const okayCerts = `
groups:
- hosts:
  - abc.test.com
  secret:
    name: test-a
    namespace: test-a
- hosts:
  - xyz.test.com
  secret:
    name: test-b
    namespace: test-b
- hosts:
  - "*.test.com"
  secret:
    name: test-c
    namespace: test-c
`

const errorCerts = `
groups:
- hosts:
  - abc.test.com
  secret:
    name: test-a
    namespace: test-a
- hosts:
  - "*.*.test.com"
  secret:
    name: test-b
    namespace: test-b
`
