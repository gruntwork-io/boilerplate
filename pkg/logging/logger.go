// Package logging provides a level-gated logger interface for boilerplate.
//
// The standard implementation wraps stdlib log.Logger rather than log/slog so the on-the-wire
// format stays identical to historical boilerplate output ("[boilerplate] 2006/01/02 15:04:05
// message"), which CLI users already depend on.
//
// Each level method emits exactly one log record per call via log.Printf, so a writer passed
// to New receives one Write per record and may split on writes without re-buffering.
package logging

import (
	"fmt"
	"io"
	"log"
)

// Logger receives boilerplate's diagnostic output. Implementations must be safe for
// concurrent use. Construct one with New, or implement the interface directly to forward
// records into a structured logger.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Level controls which records are written. Records at or above the configured level are
// emitted; the rest are dropped.
type Level int

// Levels in ascending order of severity.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

const (
	logPrefix = "[boilerplate] "
	logFlags  = log.LstdFlags
)

// String returns the lowercase name of the level (debug, info, warn, error). Unknown values
// render as "level(N)" so misuse is visible in output rather than silent.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return fmt.Sprintf("level(%d)", int(l))
	}
}

// New returns a Logger that writes to w at the given minimum level, using the boilerplate
// prefix and stdlib timestamp format. Passing a nil writer panics; the returned Logger is
// immutable, so callers wanting to change writer or level should construct a new one.
func New(w io.Writer, l Level) Logger {
	if w == nil {
		panic("logging: New called with nil writer")
	}

	return &stdLogger{
		log:   log.New(w, logPrefix, logFlags),
		level: l,
	}
}

// Discard returns a Logger that drops every record. Useful in tests and for callers that
// want boilerplate to stay silent.
func Discard() Logger {
	return discardLogger{}
}

type stdLogger struct {
	log   *log.Logger
	level Level
}

func (s *stdLogger) Debugf(format string, args ...any) {
	s.logAt(LevelDebug, format, args...)
}

func (s *stdLogger) Infof(format string, args ...any) {
	s.logAt(LevelInfo, format, args...)
}

func (s *stdLogger) Warnf(format string, args ...any) {
	s.logAt(LevelWarn, format, args...)
}

func (s *stdLogger) Errorf(format string, args ...any) {
	s.logAt(LevelError, format, args...)
}

func (s *stdLogger) logAt(at Level, format string, args ...any) {
	if at < s.level {
		return
	}

	s.log.Printf(format, args...)
}

type discardLogger struct{}

func (discardLogger) Debugf(string, ...any) {}
func (discardLogger) Infof(string, ...any)  {}
func (discardLogger) Warnf(string, ...any)  {}
func (discardLogger) Errorf(string, ...any) {}
