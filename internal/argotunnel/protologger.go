package argotunnel

import (
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

// Setting Formatter and Out is not thread-safe; Level and Hooks
// use the loggers internal mechanism (e.g. only change Formatter
// and Out in main prior to threaded execution).

var (
	// protoLogger is the name of the proto logger
	protoLogger = func() *logrus.Logger {
		log := logrus.New()
		log.SetLevel(logrus.PanicLevel)
		log.Out = os.Stderr
		return log
	}()
)

// ProtoLogger returns the proto logger
func ProtoLogger() *logrus.Logger {
	return protoLogger
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	protoLogger.Out = out
}

// SetFormatter sets the standard logger formatter.
func SetFormatter(formatter logrus.Formatter) {
	protoLogger.Formatter = formatter
}

// SetLevel sets the standard logger level.
func SetLevel(level logrus.Level) {
	protoLogger.SetLevel(level)
}

// GetLevel returns the standard logger level.
func GetLevel() logrus.Level {
	return protoLogger.Level
}

// AddHook adds a hook to the standard logger hooks.
func AddHook(hook logrus.Hook) {
	protoLogger.Hooks.Add(hook)
}
