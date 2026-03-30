package consumer

import (
	"testing"

	"task-processor/internal/core/config"

	"github.com/sirupsen/logrus"
)

func TestRabbitMQServiceFilterQueueConfigsByRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		expected []string
	}{
		{
			name:     "task role keeps only task queues",
			role:     config.NodeRoleTask,
			expected: []string{"amazon.tasks.store.*", "shein.tasks.store.*"},
		},
		{
			name:     "crawler role keeps only crawler queues",
			role:     config.NodeRoleCrawler,
			expected: []string{"amazon.crawler", "1688.crawler"},
		},
		{
			name:     "hybrid role keeps all queues",
			role:     config.NodeRoleHybrid,
			expected: []string{"amazon.tasks.store.*", "amazon.crawler", "1688.crawler", "shein.tasks.store.*"},
		},
	}

	source := []config.QueueConfig{
		{Name: "amazon.tasks.store.*"},
		{Name: "amazon.crawler"},
		{Name: "1688.crawler"},
		{Name: "shein.tasks.store.*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewRabbitMQService(&config.RabbitMQConfig{
				URL: "amqp://guest:guest@localhost:5672/",
				Node: config.NodeConfig{
					Role: tt.role,
				},
			}, logrus.New())

			filtered := svc.filterQueueConfigsByRole(source)
			if len(filtered) != len(tt.expected) {
				t.Fatalf("expected %d queue configs, got %d", len(tt.expected), len(filtered))
			}

			for i, expected := range tt.expected {
				if filtered[i].Name != expected {
					t.Fatalf("expected queue %q at index %d, got %q", expected, i, filtered[i].Name)
				}
			}
		})
	}
}
