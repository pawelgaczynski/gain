// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
