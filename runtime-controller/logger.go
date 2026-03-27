package main

import (
	"fmt"
	"log"
	"strings"
)

type logLevel int

const (
	levelError logLevel = iota
	levelWarn
	levelInfo
	levelDebug
)

var currentLogLevel = levelInfo

func configureLogLevel(value string) error {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "info":
		currentLogLevel = levelInfo
	case "error":
		currentLogLevel = levelError
	case "warn", "warning":
		currentLogLevel = levelWarn
	case "debug":
		currentLogLevel = levelDebug
	default:
		return fmt.Errorf("unknown log level: %s", value)
	}
	return nil
}

func debugf(format string, args ...any) {
	logf(levelDebug, "DEBUG", format, args...)
}

func infof(format string, args ...any) {
	logf(levelInfo, "INFO", format, args...)
}

func warnf(format string, args ...any) {
	logf(levelWarn, "WARN", format, args...)
}

func fatalf(format string, args ...any) {
	log.Fatalf("[FATAL] "+format, args...)
}

func logf(level logLevel, label, format string, args ...any) {
	if level > currentLogLevel {
		return
	}
	log.Printf("["+label+"] "+format, args...)
}
