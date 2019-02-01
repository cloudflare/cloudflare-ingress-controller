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
	// transportLogger is the name of the TransportLogger
	transportLogger = func() *logrus.Logger {
		log := logrus.New()
		log.SetLevel(logrus.PanicLevel)
		log.Out = os.Stderr
		return log
	}()
)

// TransportLogger returns the proto logger
func TransportLogger() *logrus.Logger {
	return transportLogger
}

// SetOutput sets the standard logger output.
func SetOutput(out io.Writer) {
	transportLogger.Out = out
}

// SetFormatter sets the standard logger formatter.
func SetFormatter(formatter logrus.Formatter) {
	transportLogger.Formatter = formatter
}

// SetLevel sets the standard logger level.
func SetLevel(level logrus.Level) {
	transportLogger.SetLevel(level)
}

// GetLevel returns the standard logger level.
func GetLevel() logrus.Level {
	return transportLogger.Level
}

// AddHook adds a hook to the standard logger hooks.
func AddHook(hook logrus.Hook) {
	transportLogger.Hooks.Add(hook)
}
