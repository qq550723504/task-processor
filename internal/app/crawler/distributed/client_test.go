package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPublisher 用于测试的发布器 mock
type mockPublisher struct {
	publishErr error
	published  []publishedMsg
}

type publishedMsg struct {
	queueName string
	body      []byte
}

func (m *mockPublisher) Publish(_ context.Context, queueName string, body []byte, _ uint8) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, publishedMsg{queueName: queueName, body: body})
	return nil
}

// newTestClient 创建用于测试的客户端（注入 mock 依赖）
func newTestClient(pub Publisher, declarer QueueDeclarer, timeout time.Duration) *DistributedCrawlerClient {
	registry := NewPendingRegistry(timeout)
	listener := NewResultListener(declarer, registry, logrus.New())
	return &DistributedCrawlerClient{
		publisher:   pub,
		listener:    listener,
		registry:    registry,
		taskAdapter: newTestMessageAdapter(),
		queueNaming: newTestNamingService(),
		logger:      logrus.New(),
	}
}

// newTestMessageAdapter 创建测试用 MessageAdapter（避免依赖真实实现）
func newTestMessageAdapter() interface {
	CalculatePriority(priority int) uint8
} {
	return &testAdapter{}
}

type testAdapter struct{}

func (a *testAdapter) CalculatePriority(priority int) uint8 {
	if priority >= 8 {
		return 8
	}
	return 5
}

// newTestNamingService 创建测试用命名服务
func newTestNamingService() queueNamer {
	return &testNaming{}
}

type testNaming struct{}

func (n *testNaming) BuildCrawlerQueueName(platform string, priority int) string {
	return fmt.Sprintf("%s.crawler", platform)
}

func (n *testNaming) BuildCrawlerQueueNameByRegion(platform, region string, priority int) string {
	return fmt.Sprintf("%s.crawler.%s", platform, strings.ToLower(region))
}

func TestDistributedCrawlerClient_SubmitCrawlTask_Success(t *testing.T) {
	pub := &mockPublisher{}
	mock := newMockDeclarer()
	client := newTestClient(pub, mock, 5*time.Second)

	req := &CrawlRequest{
		TaskID:    "42",
		Platform:  "amazon",
		ProductID: "B001TEST",
		Priority:  5,
	}

	// 在后台模拟爬虫处理器返回结果
	go func() {
		time.Sleep(50 * time.Millisecond)
		// 等待监听器启动
		for client.listener.QueueName() == "" {
			time.Sleep(10 * time.Millisecond)
		}
		client.registry.Deliver(&CrawlResult{TaskID: "42", Success: true, NodeID: "test"})
	}()

	result, err := client.SubmitCrawlTask(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 验证消息被发布
	require.Len(t, pub.published, 1)
	assert.Contains(t, pub.published[0].queueName, "amazon.crawler")

	// 验证消息体包含 reply_to
	var msg map[string]any
	require.NoError(t, json.Unmarshal(pub.published[0].body, &msg))
	payload := msg["payload"].(map[string]any)
	assert.NotEmpty(t, payload["reply_to"])
}

func TestDistributedCrawlerClient_SubmitCrawlTask_ListenerStartFail(t *testing.T) {
	pub := &mockPublisher{}
	mock := newMockDeclarer()
	mock.declareErr = fmt.Errorf("RabbitMQ 不可用")
	client := newTestClient(pub, mock, 5*time.Second)

	_, err := client.SubmitCrawlTask(context.Background(), &CrawlRequest{TaskID: "1"})
	assert.ErrorContains(t, err, "启动结果监听器失败")
	// 没有消息被发布
	assert.Empty(t, pub.published)
}

func TestDistributedCrawlerClient_SubmitCrawlTask_PublishFail(t *testing.T) {
	pub := &mockPublisher{publishErr: fmt.Errorf("网络错误")}
	mock := newMockDeclarer()
	client := newTestClient(pub, mock, 5*time.Second)

	_, err := client.SubmitCrawlTask(context.Background(), &CrawlRequest{TaskID: "2"})
	assert.ErrorContains(t, err, "发布爬虫任务失败")
	// 任务应从 registry 中清理
	assert.Equal(t, 0, client.registry.Len())
}

func TestDistributedCrawlerClient_SubmitCrawlTask_Timeout(t *testing.T) {
	pub := &mockPublisher{}
	mock := newMockDeclarer()
	client := newTestClient(pub, mock, 50*time.Millisecond)

	_, err := client.SubmitCrawlTask(context.Background(), &CrawlRequest{TaskID: "3"})
	assert.ErrorContains(t, err, "爬虫任务超时")
}

func TestDistributedCrawlerClient_EnsureListenerStarted_Idempotent(t *testing.T) {
	pub := &mockPublisher{}
	mock := newMockDeclarer()
	client := newTestClient(pub, mock, 5*time.Second)

	name1, err := client.ensureListenerStarted()
	require.NoError(t, err)

	name2, err := client.ensureListenerStarted()
	require.NoError(t, err)

	assert.Equal(t, name1, name2)
}

func TestResultListener_Restart(t *testing.T) {
	mock := newMockDeclarer()
	reg := NewPendingRegistry(5 * time.Second)
	l := NewResultListener(mock, reg, logrus.New())

	name1, err := l.Start()
	require.NoError(t, err)

	// 模拟重连：重置并重新启动
	// 关闭旧的 delivery channel，触发 consume goroutine 退出
	close(mock.deliveryCh)
	mock.deliveryCh = make(chan amqp.Delivery, 10)

	err = l.Restart()
	require.NoError(t, err)

	name2 := l.QueueName()
	// Restart 后队列名会重新生成（时间戳不同）
	assert.NotEmpty(t, name2)
	_ = name1
}
