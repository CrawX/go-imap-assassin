// SPDX-License-Identifier: GPL-3.0-or-later
package log

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var loggers map[string]*logrus.Logger

func NewPrefixLogger(prefix string) *PrefixLogger {
	stringPrefix := fmt.Sprintf("%s:\t", prefix)

	formatter := &logrus.TextFormatter{}
	formatter.FullTimestamp = true
	formatter.TimestampFormat = "15:04:05"
	formatter.DisableColors = strings.Contains(runtime.GOOS, "windows")
	return &PrefixLogger{
		formatter,
		[]byte(stringPrefix),
	}
}

type PrefixLogger struct {
	formatter logrus.Formatter
	prefix    []byte
}

func (f *PrefixLogger) Format(entry *logrus.Entry) ([]byte, error) {
	text, err := f.formatter.Format(entry)
	if err != nil {
		return nil, err
	}
	return append(f.prefix, text...), nil
}

const (
	LOG_MAIN         = "MA"
	LOG_IMAPASSASSIN = "IA"
	LOG_SPAMASSASSIN = "SA"
	LOG_PERSISTENCE  = "PI"
	LOG_IMAP         = "IM"
)

func getLevel(loglevel string) logrus.Level {
	switch strings.ToLower(loglevel) {
	case "debug":
		return logrus.DebugLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "panic":
		return logrus.PanicLevel
	case "fatal":
		return logrus.FatalLevel
	}

	// Info is default
	return logrus.InfoLevel
}

func initLogger(prefix, loglevel string) {
	loggers[prefix] = logrus.New()
	loggers[prefix].Level = getLevel(loglevel)
	loggers[prefix].Formatter = NewPrefixLogger(prefix)
}

func InitLogging(loglevel string) {
	loggers = make(map[string]*logrus.Logger)
	for _, prefix := range []string{
		LOG_MAIN,
		LOG_IMAPASSASSIN,
		LOG_SPAMASSASSIN,
		LOG_PERSISTENCE,
		LOG_IMAP,
	} {
		initLogger(prefix, loglevel)
	}

}

func SetLogLevel(loglevel string) {
	for _, v := range loggers {
		v.Level = getLevel(loglevel)
	}
}

func Logger(logger string) *logrus.Logger {
	l, ok := loggers[logger]
	if !ok {
		panic("Logger " + logger + " unknown")
	}

	return l
}
