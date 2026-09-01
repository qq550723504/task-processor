package rabbitmq

import "testing"

func TestBuildTaskQueueName(t *testing.T) {
	naming := NewNamingService()

	if got := naming.BuildTaskQueueName("shein", 1); got != "shein.tasks" {
		t.Fatalf("expected shared queue name, got %q", got)
	}
}

func TestBuildTaskQueueNameForStore(t *testing.T) {
	naming := NewNamingService()

	tests := []struct {
		name     string
		platform string
		storeID  int64
		want     string
	}{
		{
			name:     "routes to store queue when store id is set",
			platform: "shein",
			storeID:  123,
			want:     "shein.tasks.store.123",
		},
		{
			name:     "falls back to shared queue when store id is missing",
			platform: "shein",
			storeID:  0,
			want:     "shein.tasks",
		},
		{
			name:     "normalizes crawler platform to base platform",
			platform: "shein.crawler",
			storeID:  456,
			want:     "shein.tasks.store.456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := naming.BuildTaskQueueNameForStore(tt.platform, 1, tt.storeID); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
