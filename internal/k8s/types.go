package k8s

import (
	"fmt"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	objDelim = "/"
)

// ObjMixin is a kingpin convenience function
func ObjMixin(s kingpin.Settings) (val *ObjValue) {
	val = &ObjValue{}
	s.SetValue(val)
	return
}

// Obj is a name namespace tuple
type Obj struct {
	Name, Namespace string
}

// ObjValue is a meta namespace key variant implementing the Value interface
type ObjValue Obj

// Set values the object from a string or errors.
func (o *ObjValue) Set(val string) error {
	if len(val) > 0 {
		parts := strings.SplitN(strings.TrimSpace(val), objDelim, 3)
		if len(parts) != 2 {
			return fmt.Errorf("expected '<namespace>/<name>' got '%s'", val)
		} else if parts[0] == "" {
			return fmt.Errorf("expected '<namespace>/<name>' got '%s'", val)
		} else if parts[1] == "" {
			return fmt.Errorf("expected '<namespace>/<name>' got '%s'", val)
		}
		o.Namespace, o.Name = parts[0], parts[1]
	}
	return nil
}

func (o *ObjValue) String() (val string) {
	if o.Namespace != "" && o.Name != "" {
		val = o.Namespace + objDelim + o.Name
	}
	return
}
