package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/alecthomas/kingpin.v2"
)

func TestObjMixin(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  string
		out *ObjValue
		err error
	}{
		"obj-empty": {
			in:  "",
			out: &ObjValue{},
			err: nil,
		},
		"obj-no-name": {
			in:  "a/",
			out: &ObjValue{},
			err: fmt.Errorf("expected '<namespace>/<name>' got '%s'", "a/"),
		},
		"obj-no-namespace": {
			in:  "/b",
			out: &ObjValue{},
			err: fmt.Errorf("expected '<namespace>/<name>' got '%s'", "/b"),
		},
		"obj-too-many-parts": {
			in:  "a/b/c",
			out: &ObjValue{},
			err: fmt.Errorf("expected '<namespace>/<name>' got '%s'", "a/b/c"),
		},
		"obj-okay": {
			in: "a/b",
			out: &ObjValue{
				Namespace: "a",
				Name:      "b",
			},
			err: nil,
		},
	} {
		app := kingpin.New(name, name)
		out := ObjMixin(app.Flag("o", "test"))
		_, err := app.Parse([]string{"--o=" + test.in})
		assert.Equalf(t, test.out, out, "test '%s' obj mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' err mismatch", name)
	}
}
