package logging

import "github.com/rs/zerolog"

// Nop returns a logger that discards all output (tests).
func Nop() zerolog.Logger {
	return zerolog.Nop()
}
