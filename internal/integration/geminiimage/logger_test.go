package geminiimage

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestAdaptLogrusIsNilSafeAndForwardsStructuredFields(t *testing.T) {
	AdaptLogrus(nil).Debug("nil-safe", nil)
	base := logrus.New()
	var output bytes.Buffer
	base.SetOutput(&output)
	base.SetFormatter(&logrus.JSONFormatter{})
	AdaptLogrus(logrus.NewEntry(base)).Info("provider-log", map[string]any{"provider": "geminiimage"})
	got := output.String()
	if !strings.Contains(got, `"msg":"provider-log"`) || !strings.Contains(got, `"provider":"geminiimage"`) {
		t.Fatalf("structured log output = %s", got)
	}
}
