package grsai

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

type recordingLogger struct {
	messages []string
	fields   []map[string]any
}

func TestClientDiagnosticUsesInjectedLogger(t *testing.T) {
	recorder := &recordingLogger{}
	client := NewClient(Config{Logger: recorder, Model: "model", Timeout: time.Second})
	client.logSubmitDiagnostic(context.Background(), "https://provider.example/v1", submitRequest{Model: "model", Size: "1024x1024"}, "generation")
	if len(recorder.messages) != 1 || recorder.messages[0] != "grsai submit diagnostic" {
		t.Fatalf("messages = %#v", recorder.messages)
	}
	fields := recorder.fields[0]
	if fields["mode"] != "generation" || fields["model"] != "model" || fields["submit_url"] != "https://provider.example/v1" {
		t.Fatalf("fields = %#v", fields)
	}
}

func (l *recordingLogger) record(message string, fields map[string]any) {
	l.messages = append(l.messages, message)
	l.fields = append(l.fields, fields)
}
func (l *recordingLogger) Debug(message string, fields map[string]any) { l.record(message, fields) }
func (l *recordingLogger) Info(message string, fields map[string]any)  { l.record(message, fields) }
func (l *recordingLogger) Warn(message string, fields map[string]any)  { l.record(message, fields) }
func (l *recordingLogger) Error(message string, fields map[string]any) { l.record(message, fields) }

func TestAdaptLogrusIsNilSafeAndForwardsStructuredFields(t *testing.T) {
	AdaptLogrus(nil).Warn("nil-safe", nil)
	base := logrus.New()
	var output bytes.Buffer
	base.SetOutput(&output)
	base.SetFormatter(&logrus.JSONFormatter{})
	AdaptLogrus(logrus.NewEntry(base)).Info("provider-log", map[string]any{"provider": "grsai"})
	got := output.String()
	if !strings.Contains(got, `"msg":"provider-log"`) || !strings.Contains(got, `"provider":"grsai"`) {
		t.Fatalf("structured log output = %s", got)
	}
}
