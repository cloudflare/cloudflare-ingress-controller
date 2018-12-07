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
type ObjValue []Obj

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
		*o = append(*o, Obj{Namespace: parts[0], Name: parts[1]})
	}
	return nil
}

func (o *ObjValue) String() (val string) {
	for i, obj := range *o {
		if obj.Namespace != "" && obj.Name != "" {
			val += fmt.Sprintf("%v%v%v", obj.Namespace, objDelim, obj.Name)
		}

		if i != len(*o)-1 {
			val += ", "
		}
	}
	return
}

func (o *ObjValue) IsCumulative() bool {
	return true
}
