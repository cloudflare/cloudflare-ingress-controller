package main

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestLogrusLevel(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  int
		out logrus.Level
	}{
		"verbose-flag-lt-0": {
			in:  -100,
			out: logrus.PanicLevel,
		},
		"verbose-flag-0": {
			in:  0,
			out: logrus.PanicLevel,
		},
		"verbose-flag-1": {
			in:  1,
			out: logrus.FatalLevel,
		},
		"verbose-flag-2": {
			in:  2,
			out: logrus.ErrorLevel,
		},
		"verbose-flag-3": {
			in:  3,
			out: logrus.WarnLevel,
		},
		"verbose-flag-4": {
			in:  4,
			out: logrus.InfoLevel,
		},
		"verbose-flag-5": {
			in:  5,
			out: logrus.DebugLevel,
		},
		"verbose-flag-100": {
			in:  100,
			out: logrus.DebugLevel,
		},
	} {
		out := logruslevel(test.in)
		assert.Equalf(t, test.out, out, "test '%s' logrus level mismatch", name)
	}
}
