package main

import (
	"flag"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type Level int32

// set sets the value of the Level.
func (l *Level) set(val Level) {
	atomic.StoreInt32((*int32)(l), int32(val))
}

// String is part of the flag.Value interface.
func (l *Level) String() string {
	return strconv.FormatInt(int64(*l), 10)
}

// Get is part of the flag.Value interface.
func (l *Level) Get() interface{} {
	return *l
}

// Set is part of the flag.Value interface.
func (l *Level) Set(value string) error {
	v, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	l.set(Level(v))
	return nil
}
func TestLogLevel(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		in  []string
		out logrus.Level
	}{
		"verbose-flag-missing": {
			in:  []string{},
			out: logrus.PanicLevel,
		},
		"verbose-flag-0": {
			in:  []string{"--v=0"},
			out: logrus.PanicLevel,
		},
		"verbose-flag-1": {
			in:  []string{"--v=1"},
			out: logrus.FatalLevel,
		},
		"verbose-flag-2": {
			in:  []string{"--v=2"},
			out: logrus.ErrorLevel,
		},
		"verbose-flag-3": {
			in:  []string{"--v=3"},
			out: logrus.WarnLevel,
		},
		"verbose-flag-4": {
			in:  []string{"--v=4"},
			out: logrus.InfoLevel,
		},
		"verbose-flag-5": {
			in:  []string{"--v=5"},
			out: logrus.DebugLevel,
		},
		"verbose-flag-100": {
			in:  []string{"--v=100"},
			out: logrus.DebugLevel,
		},
	} {
		var v Level
		flags := flag.NewFlagSet(name, flag.ContinueOnError)
		flags.Var(&v, "v", "log level for V logs")
		flags.Parse(test.in)

		out := loglevel(flags)
		assert.Equalf(t, test.out, out, "test '%s' loglevel mismatch", name)
	}
}
