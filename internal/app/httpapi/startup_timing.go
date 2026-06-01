package httpapi

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type startupTimer struct {
	logger    *logrus.Logger
	start     time.Time
	startupID string
}

func newStartupTimer(logger *logrus.Logger) *startupTimer {
	start := time.Now().UTC()
	timer := &startupTimer{
		logger:    logger,
		start:     start,
		startupID: fmt.Sprintf("%d-%d", os.Getpid(), start.UnixNano()),
	}
	timer.log("startup begin", "bootstrap", time.Duration(0))
	return timer
}

func (t *startupTimer) phase(name string) func() {
	begin := time.Now()
	return func() {
		t.log("startup phase", name, time.Since(begin))
	}
}

func (t *startupTimer) total(name string) {
	t.log("startup total", name, time.Since(t.start))
}

func (t *startupTimer) log(message string, phase string, elapsed time.Duration) {
	if t == nil || t.logger == nil {
		return
	}
	t.logger.WithFields(logrus.Fields{
		"startup_id": t.startupID,
		"phase":      phase,
		"elapsed":    elapsed.String(),
	}).Info(message)
}
