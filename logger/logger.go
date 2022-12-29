package logger

import (
	"os"

	"github.com/rs/zerolog"
)

const (
	DebugLevel = zerolog.DebugLevel
	InfoLevel  = zerolog.InfoLevel
	WarnLevel  = zerolog.WarnLevel
	ErrorLevel = zerolog.ErrorLevel
	FatalLevel = zerolog.FatalLevel
	PanicLevel = zerolog.PanicLevel
	Disabled   = zerolog.Disabled
	TraceLevel = zerolog.TraceLevel
	NoLevel    = zerolog.NoLevel
)

func NewLogger(component string, level zerolog.Level, pretty bool) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logger := zerolog.New(os.Stdout).With().Timestamp().Str("component", component).Logger().Level(level)

	if pretty {
		logger = logger.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	return logger
}
