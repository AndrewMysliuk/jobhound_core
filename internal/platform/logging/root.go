package logging

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// NewRoot builds a process-level logger with timestamp, binary name as service, and level/format from config strings.
func NewRoot(levelStr, formatStr, binaryName string) zerolog.Logger {
	level := parseLevel(levelStr)
	out := io.Writer(os.Stdout)
	binaryName = strings.TrimSpace(binaryName)
	var zl zerolog.Logger
	switch strings.ToLower(strings.TrimSpace(formatStr)) {
	case "json":
		zl = zerolog.New(out).Level(level).With().Timestamp().Str(FieldService, binaryName).Logger()
	default:
		cw := zerolog.ConsoleWriter{Out: out, TimeFormat: time.RFC3339}
		zl = zerolog.New(cw).Level(level).With().Timestamp().Str(FieldService, binaryName).Logger()
	}
	return zl
}

func parseLevel(s string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "disabled", "off", "none":
		return zerolog.Disabled
	}
	l, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(s)))
	if err != nil {
		return zerolog.InfoLevel
	}
	return l
}
