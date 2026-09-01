package openai

import "github.com/sirupsen/logrus"

type Logger interface {
	Debug(message string, fields map[string]any)
	Info(message string, fields map[string]any)
	Warn(message string, fields map[string]any)
	Error(message string, fields map[string]any)
}

type noopLogger struct{}

func (noopLogger) Debug(string, map[string]any) {}
func (noopLogger) Info(string, map[string]any)  {}
func (noopLogger) Warn(string, map[string]any)  {}
func (noopLogger) Error(string, map[string]any) {}

type logrusAdapter struct{ entry *logrus.Entry }

func AdaptLogrus(entry *logrus.Entry) Logger {
	if entry == nil {
		return noopLogger{}
	}
	return logrusAdapter{entry: entry}
}

func (l logrusAdapter) Debug(message string, fields map[string]any) {
	l.entry.WithFields(fields).Debug(message)
}
func (l logrusAdapter) Info(message string, fields map[string]any) {
	l.entry.WithFields(fields).Info(message)
}
func (l logrusAdapter) Warn(message string, fields map[string]any) {
	l.entry.WithFields(fields).Warn(message)
}
func (l logrusAdapter) Error(message string, fields map[string]any) {
	l.entry.WithFields(fields).Error(message)
}

func loggerOrNoop(logger Logger) Logger {
	if logger == nil {
		return noopLogger{}
	}
	return logger
}
