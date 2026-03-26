package distributed

import (
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueueDeclarer 用于测试的 mock 实现
type mockQueueDeclarer struct {
	declareErr   error
	consumeErr   error
	deliveryCh   chan amqp.Delivery
	declaredName string
}

func newMockDeclarer() *mockQueueDeclarer {
	return &mockQueueDeclarer{
		deliveryCh: make(chan amqp.Delivery, 10),
	}
}

func (m *mockQueueDeclarer) DeclareExclusiveQueue(name string) (string, error) {
	if m.declareErr != nil {
		return "", m.declareErr
	}
	m.declaredName = name
	return name, nil
}

func (m *mockQueueDeclarer) ConsumeQueue(queueName, consumerTag string) (<-chan amqp.Delivery, error) {
	if m.consumeErr != nil {
		return nil, m.consumeErr
	}
	return m.deliveryCh, nil
}

func TestResultListener_Start_Success(t *testing.T) {
	mock := newMockDeclarer()
	reg := NewPendingRegistry(5 * time.Second)
	l := NewResultListener(mock, reg, logrus.New())

	name, err := l.Start()
	require.NoError(t, err)
	assert.NotEmpty(t, name)
	assert.Equal(t, name, l.QueueName())
}

func TestResultListener_Start_Idempotent(t *testing.T) {
	mock := newMockDeclarer()
	reg := NewPendingRegistry(5 * time.Second)
	l := NewResultListener(mock, reg, logrus.New())

	name1, err := l.Start()
	require.NoError(t, err)

	name2, err := l.Start()
	require.NoError(t, err)

	// 第二次调用应返回相同队列名，不重新声明
	assert.Equal(t, name1, name2)
}

func TestResultListener_Start_DeclareError(t *testing.T) {
	mock := newMockDeclarer()
	mock.declareErr = assert.AnError
	reg := NewPendingRegistry(5 * time.Second)
	l := NewResultListener(mock, reg, logrus.New())

	_, err := l.Start()
	assert.Error(t, err)
	assert.Empty(t, l.QueueName())
}

func TestResultListener_HandleMessage_DeliverResult(t *testing.T) {
	mock := newMockDeclarer()
	reg := NewPendingRegistry(5 * time.Second)
	l := NewResultListener(mock, reg, logrus.New())

	pt := reg.Register(t.Context(), "100")

	body := `{"taskId":"100","success":true,"nodeId":"test-node"}`
	msg := amqp.Delivery{Body: []byte(body)}
	l.handleMessage(msg)

	result, err := reg.Wait(pt)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "100", result.TaskID)
}

func TestResultListener_HandleMessage_InvalidJSON(t *testing.T) {
	mock := newMockDeclarer()
	reg := NewPendingRegistry(5 * time.Second)
	l := NewResultListener(mock, reg, logrus.New())

	// 无效 JSON 不应 panic
	msg := amqp.Delivery{Body: []byte("not-json")}
	assert.NotPanics(t, func() { l.handleMessage(msg) })
}

func TestResultListener_HandleMessage_UnknownTask(t *testing.T) {
	mock := newMockDeclarer()
	reg := NewPendingRegistry(5 * time.Second)
	l := NewResultListener(mock, reg, logrus.New())

	// 没有等待的任务，不应 panic
	body := `{"taskId":"999","success":false}`
	msg := amqp.Delivery{Body: []byte(body)}
	assert.NotPanics(t, func() { l.handleMessage(msg) })
}
