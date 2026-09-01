package lifecycle

import (
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLifecycleManagerStartsDependenciesBeforeDependents(t *testing.T) {
	events := make([]string, 0, 3)
	m := NewLifecycleManager(logrus.New())
	components := []*recordingComponent{
		newRecordingComponent("database", nil, 30, &events),
		newRecordingComponent("worker", []string{"database"}, 20, &events),
		newRecordingComponent("api", []string{"worker"}, 10, &events),
	}
	for _, component := range components {
		if err := m.Register(component); err != nil {
			t.Fatal(err)
		}
	}

	if err := m.StartAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"start:database", "start:worker", "start:api"}
	if got := strings.Join(events, ","); got != strings.Join(want, ",") {
		t.Fatalf("start order = %v, want %v", events, want)
	}
}

func TestLifecycleManagerStopsDependentsBeforeDependencies(t *testing.T) {
	events := make([]string, 0, 6)
	m := NewLifecycleManager(logrus.New())
	components := []*recordingComponent{
		newRecordingComponent("database", nil, 30, &events),
		newRecordingComponent("worker", []string{"database"}, 20, &events),
		newRecordingComponent("api", []string{"worker"}, 10, &events),
	}
	for _, component := range components {
		if err := m.Register(component); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.StartAll(context.Background()); err != nil {
		t.Fatal(err)
	}
	events = events[:0]

	if err := m.StopAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"stop:api", "stop:worker", "stop:database"}
	if got := strings.Join(events, ","); got != strings.Join(want, ",") {
		t.Fatalf("stop order = %v, want %v", events, want)
	}
}

func TestLifecycleManagerOrdersIndependentComponentsByPriority(t *testing.T) {
	events := make([]string, 0, 3)
	m := NewLifecycleManager(logrus.New())
	components := []*recordingComponent{
		newRecordingComponent("low", nil, 30, &events),
		newRecordingComponent("high", nil, 10, &events),
		newRecordingComponent("medium", nil, 20, &events),
	}
	for _, component := range components {
		if err := m.Register(component); err != nil {
			t.Fatal(err)
		}
	}

	if err := m.StartAll(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"start:high", "start:medium", "start:low"}
	if got := strings.Join(events, ","); got != strings.Join(want, ",") {
		t.Fatalf("start order = %v, want %v", events, want)
	}
}

func TestLifecycleManagerRejectsDependencyCycle(t *testing.T) {
	m := NewLifecycleManager(logrus.New())
	if err := m.Register(newTestComponent("a", []string{"b"}, 10)); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(newTestComponent("b", []string{"a"}, 20)); err != nil {
		t.Fatal(err)
	}
	if err := m.StartAll(context.Background()); err == nil || !strings.Contains(err.Error(), "循环依赖") {
		t.Fatalf("StartAll() error = %v", err)
	}
}

type testComponent struct {
	*BaseComponent
}

func newTestComponent(name string, dependencies []string, priority int) *testComponent {
	return &testComponent{BaseComponent: NewBaseComponent(name, dependencies, priority)}
}

func (c *testComponent) Start(context.Context) error {
	c.SetRunning(true)
	return nil
}

func (c *testComponent) Stop(context.Context) error {
	c.SetRunning(false)
	return nil
}

type recordingComponent struct {
	*BaseComponent
	events *[]string
}

func newRecordingComponent(name string, dependencies []string, priority int, events *[]string) *recordingComponent {
	return &recordingComponent{
		BaseComponent: NewBaseComponent(name, dependencies, priority),
		events:        events,
	}
}

func (c *recordingComponent) Start(context.Context) error {
	*c.events = append(*c.events, "start:"+c.Name())
	c.SetRunning(true)
	return nil
}

func (c *recordingComponent) Stop(context.Context) error {
	*c.events = append(*c.events, "stop:"+c.Name())
	c.SetRunning(false)
	return nil
}
