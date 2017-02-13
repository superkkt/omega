package logger

import (
	"fmt"
	"log"
	"os"
	"sync"
)

var (
	mu     sync.RWMutex
	level  Level
	writer *log.Logger
	prefix func() string
)

type Level uint8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarning
	LevelError
	LevelFatal
)

func (r Level) String() string {
	switch r {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func SetLogger(w *log.Logger) {
	mu.Lock()
	defer mu.Unlock()

	writer = w
}

func SetLogLevel(v Level) {
	mu.Lock()
	defer mu.Unlock()

	level = v
}

func SetPrefix(f func() string) {
	mu.Lock()
	defer mu.Unlock()

	prefix = f
}

func write(l Level, msg string) {
	mu.RLock()
	defer mu.RUnlock()

	if level > l {
		return
	}
	if writer == nil {
		fmt.Println(msg)
		return
	}

	var v string
	if prefix != nil {
		v = fmt.Sprintf("%v: %v%v", l, prefix(), msg)
	} else {
		v = fmt.Sprintf("%v: %v", l, msg)
	}
	writer.Println(v)
}

func Debug(m string) {
	write(LevelDebug, m)
}

func Info(m string) {
	write(LevelInfo, m)
}

func Warning(m string) {
	write(LevelWarning, m)
}

func Error(m string) {
	write(LevelError, m)
}

func Fatal(m string) {
	write(LevelFatal, m)
	os.Exit(1)
}
