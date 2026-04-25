// Package logging provides a process-global, level-gated logger for boilerplate.
//
// The implementation wraps stdlib log.Logger rather than log/slog so the on-the-wire format
// stays identical to historical boilerplate output ("[boilerplate] 2006/01/02 15:04:05 message"),
// which CLI users already depend on.
//
// Each level method (Debugf, Infof, Warnf, Errorf) emits exactly one log record per call via
// log.Printf, so a writer set with SetWriter receives one Write per record and may split on
// writes without re-buffering.
package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

// Level controls which records are written. Records at or above the configured level are emitted;
// the rest are dropped.
type Level int

// Levels in ascending order of severity. Use these with SetLevel and compare against CurrentLevel.
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

// Logger is a level-gated wrapper around stdlib log.Logger. The zero value is not usable; obtain
// an instance with New. A Logger is safe for concurrent use.
type Logger struct {
	log    *log.Logger
	writer io.Writer
	mu     sync.RWMutex
	level  Level
}

var defaultLogger = New(os.Stdout, LevelInfo)

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

// New constructs a Logger that writes to w at the given minimum level, using the standard
// boilerplate prefix and timestamp format.
func New(w io.Writer, l Level) *Logger {
	return &Logger{
		log:    log.New(w, logPrefix, logFlags),
		writer: w,
		level:  l,
	}
}

// SetWriter redirects the default logger's output to w. The writer receives one Write per
// emitted record, so adapters that split lines need not buffer across calls. Passing nil panics.
func SetWriter(w io.Writer) {
	defaultLogger.SetWriter(w)
}

// SetLevel sets the minimum severity emitted by the default logger. Records below l are dropped.
func SetLevel(l Level) {
	defaultLogger.SetLevel(l)
}

// CurrentWriter returns the io.Writer currently receiving output from the default logger.
func CurrentWriter() io.Writer {
	return defaultLogger.CurrentWriter()
}

// CurrentLevel returns the minimum severity currently configured on the default logger.
func CurrentLevel() Level {
	return defaultLogger.CurrentLevel()
}

// Debugf emits a record at Debug severity through the default logger.
func Debugf(format string, args ...any) {
	defaultLogger.logAt(LevelDebug, format, args...)
}

// Infof emits a record at Info severity through the default logger.
func Infof(format string, args ...any) {
	defaultLogger.logAt(LevelInfo, format, args...)
}

// Warnf emits a record at Warn severity through the default logger.
func Warnf(format string, args ...any) {
	defaultLogger.logAt(LevelWarn, format, args...)
}

// Errorf emits a record at Error severity through the default logger.
func Errorf(format string, args ...any) {
	defaultLogger.logAt(LevelError, format, args...)
}

// SetWriter redirects this logger's output to w. Passing nil panics.
func (lg *Logger) SetWriter(w io.Writer) {
	if w == nil {
		panic("logging: SetWriter called with nil writer")
	}

	lg.mu.Lock()
	defer lg.mu.Unlock()

	lg.writer = w
	lg.log = log.New(w, logPrefix, logFlags)
}

// SetLevel sets this logger's minimum severity.
func (lg *Logger) SetLevel(l Level) {
	lg.mu.Lock()
	defer lg.mu.Unlock()

	lg.level = l
}

// CurrentWriter returns this logger's current writer.
func (lg *Logger) CurrentWriter() io.Writer {
	lg.mu.RLock()
	defer lg.mu.RUnlock()

	return lg.writer
}

// CurrentLevel returns this logger's current minimum severity.
func (lg *Logger) CurrentLevel() Level {
	lg.mu.RLock()
	defer lg.mu.RUnlock()

	return lg.level
}

// Debugf emits a record at Debug severity through this logger.
func (lg *Logger) Debugf(format string, args ...any) {
	lg.logAt(LevelDebug, format, args...)
}

// Infof emits a record at Info severity through this logger.
func (lg *Logger) Infof(format string, args ...any) {
	lg.logAt(LevelInfo, format, args...)
}

// Warnf emits a record at Warn severity through this logger.
func (lg *Logger) Warnf(format string, args ...any) {
	lg.logAt(LevelWarn, format, args...)
}

// Errorf emits a record at Error severity through this logger.
func (lg *Logger) Errorf(format string, args ...any) {
	lg.logAt(LevelError, format, args...)
}

func (lg *Logger) logAt(l Level, format string, args ...any) {
	lg.mu.RLock()

	if l < lg.level {
		lg.mu.RUnlock()
		return
	}

	target := lg.log
	lg.mu.RUnlock()

	target.Printf(format, args...)
}
