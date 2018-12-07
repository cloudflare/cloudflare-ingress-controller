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
		in  []string
		out *ObjValue
		err error
	}{
		"obj-empty": {
			in:  []string{""},
			out: &ObjValue{},
			err: nil,
		},
		"obj-no-name": {
			in:  []string{"a/"},
			out: &ObjValue{},
			err: fmt.Errorf("expected '<namespace>/<name>' got '%s'", "a/"),
		},
		"obj-no-namespace": {
			in:  []string{"/b"},
			out: &ObjValue{},
			err: fmt.Errorf("expected '<namespace>/<name>' got '%s'", "/b"),
		},
		"obj-too-many-parts": {
			in:  []string{"a/b/c"},
			out: &ObjValue{},
			err: fmt.Errorf("expected '<namespace>/<name>' got '%s'", "a/b/c"),
		},
		"obj-okay-single": {
			in: []string{"a/b"},
			out: &ObjValue{
				Obj{
					Namespace: "a",
					Name:      "b",
				},
			},
			err: nil,
		},
		"obj-okay-multiple": {
			in: []string{"a/b", "c/d", "e/f"},
			out: &ObjValue{
				Obj{
					Namespace: "a",
					Name:      "b",
				},
				Obj{
					Namespace: "c",
					Name:      "d",
				},
				Obj{
					Namespace: "e",
					Name:      "f",
				},
			},
			err: nil,
		},
	} {
		app := kingpin.New(name, name)
		out := ObjMixin(app.Flag("o", "test"))
		flags := []string{}
		for _, flag := range test.in {
			flags = append(flags, "--o="+flag)
		}
		_, err := app.Parse(flags)
		assert.Equalf(t, test.out, out, "test '%s' obj mismatch", name)
		assert.Equalf(t, test.err, err, "test '%s' err mismatch", name)
	}
}

func TestObjString(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		obj *ObjValue
		out string
	}{
		"obj-empty": {
			obj: &ObjValue{},
			out: "",
		},
		"obj-no-name": {
			obj: &ObjValue{
				Obj{
					Namespace: "a",
				},
			},
			out: "",
		},
		"obj-no-namespace": {
			obj: &ObjValue{
				Obj{
					Name: "b",
				},
			},
			out: "",
		},
		"obj-okay-single": {
			obj: &ObjValue{
				Obj{
					Namespace: "a",
					Name:      "b",
				},
			},
			out: "a/b",
		},
		"obj-okay-multiple": {
			obj: &ObjValue{
				Obj{
					Namespace: "a",
					Name:      "b",
				},
				Obj{
					Namespace: "c",
					Name:      "d",
				},
				Obj{
					Namespace: "e",
					Name:      "f",
				},
			},
			out: "a/b, c/d, e/f",
		},
	} {
		out := test.obj.String()
		assert.Equalf(t, test.out, out, "test '%s' value mismatch", name)
	}
}
