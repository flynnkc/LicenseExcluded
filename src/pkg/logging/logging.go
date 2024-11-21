package logging

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	CRITICAL
)

func GetLevel(level string) (Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DEBUG, nil
	case "info":
		return INFO, nil
	case "warn":
		return WARN, nil
	case "error":
		return ERROR, nil
	case "critical":
		return CRITICAL, nil
	default:
		return INFO, fmt.Errorf("invalid log level %s", level)
	}
}

type Lumberjack struct {
	Level          Level
	debugLogger    *log.Logger
	infoLogger     *log.Logger
	warnLogger     *log.Logger
	errorLogger    *log.Logger
	criticalLogger *log.Logger
}

func NewLogger(level string) Lumberjack {
	out := os.Stdout

	l := Lumberjack{
		debugLogger:    log.New(out, "DEBUG - ", log.LstdFlags),
		infoLogger:     log.New(out, "INFO - ", log.LstdFlags),
		warnLogger:     log.New(out, "WARN - ", log.LstdFlags),
		errorLogger:    log.New(out, "ERROR - ", log.LstdFlags),
		criticalLogger: log.New(out, "CRITICAL - ", log.LstdFlags),
	}

	lvl, err := GetLevel(level)
	if err != nil {
		lvl = INFO
	}

	l.Level = lvl

	return l
}

func (l *Lumberjack) Debugf(format string, msg ...any) {
	if l.Level <= DEBUG {
		l.debugLogger.Printf(format, msg...)
	}
}

func (l *Lumberjack) Infof(format string, msg ...any) {
	if l.Level <= INFO {
		l.infoLogger.Printf(format, msg...)
	}
}

func (l *Lumberjack) Warnf(format string, msg ...any) {
	if l.Level <= WARN {
		l.warnLogger.Printf(format, msg...)
	}
}

func (l *Lumberjack) Errorf(format string, msg ...any) {
	if l.Level <= ERROR {
		l.errorLogger.Printf(format, msg...)
	}
}

func (l *Lumberjack) Criticalf(format string, msg ...any) {
	if l.Level <= CRITICAL {
		l.criticalLogger.Printf(format, msg...)
	}
}

func (l *Lumberjack) Debug(msg ...any) {
	if l.Level <= DEBUG {
		l.debugLogger.Println(msg...)
	}
}

func (l *Lumberjack) Info(msg ...any) {
	if l.Level <= INFO {
		l.infoLogger.Println(msg...)
	}
}

func (l *Lumberjack) Warn(msg ...any) {
	if l.Level <= WARN {
		l.warnLogger.Println(msg...)
	}
}

func (l *Lumberjack) Error(msg ...any) {
	if l.Level <= ERROR {
		l.errorLogger.Println(msg...)
	}
}

func (l *Lumberjack) Critical(msg ...any) {
	if l.Level <= CRITICAL {
		l.criticalLogger.Println(msg...)
	}
}
