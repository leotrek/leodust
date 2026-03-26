package logging

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
)

// Level controls which log messages are emitted.
type Level int32

const (
	ErrorLevel Level = iota
	WarnLevel
	InfoLevel
	DebugLevel
)

var currentLevel atomic.Int32

func init() {
	currentLevel.Store(int32(InfoLevel))
}

// ParseLevel converts a user-facing log level string into a Level value.
func ParseLevel(value string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		return InfoLevel, nil
	case "error":
		return ErrorLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "debug":
		return DebugLevel, nil
	default:
		return InfoLevel, fmt.Errorf("unknown log level: %s", value)
	}
}

// Configure updates the global logging level.
func Configure(value string) error {
	level, err := ParseLevel(value)
	if err != nil {
		return err
	}
	currentLevel.Store(int32(level))
	return nil
}

// CurrentLevel returns the active global log level.
func CurrentLevel() Level {
	return Level(currentLevel.Load())
}

// Enabled reports whether messages at the given level should be emitted.
func Enabled(level Level) bool {
	return level <= CurrentLevel()
}

func (l Level) String() string {
	switch l {
	case ErrorLevel:
		return "ERROR"
	case WarnLevel:
		return "WARN"
	case InfoLevel:
		return "INFO"
	case DebugLevel:
		return "DEBUG"
	default:
		return "INFO"
	}
}

func logf(level Level, format string, args ...any) {
	if !Enabled(level) {
		return
	}

	log.Printf("["+level.String()+"] "+format, args...)
}

// Errorf logs a message at error level.
func Errorf(format string, args ...any) {
	logf(ErrorLevel, format, args...)
}

// Warnf logs a message at warning level.
func Warnf(format string, args ...any) {
	logf(WarnLevel, format, args...)
}

// Infof logs a message at info level.
func Infof(format string, args ...any) {
	logf(InfoLevel, format, args...)
}

// Debugf logs a message at debug level.
func Debugf(format string, args ...any) {
	logf(DebugLevel, format, args...)
}

// Fatalf always emits a fatal message and exits the process.
func Fatalf(format string, args ...any) {
	log.Fatalf("[FATAL] "+format, args...)
}
