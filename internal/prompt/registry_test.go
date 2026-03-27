package prompt

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestRegistry 创建测试用 registry（discard logger）
func newTestRegistry() *registryImpl {
	log := logrus.NewEntry(logrus.New())
	log.Logger.SetOutput(io.Discard)
	r := NewRegistry(log)
	return r
}

func TestRegistry_Get_Hit(t *testing.T) {
	r := newTestRegistry()
	r.cache["foo.bar"] = "hello"

	got := r.Get("foo.bar", "fallback")
	assert.Equal(t, "hello", got)
}

func TestRegistry_Get_Miss(t *testing.T) {
	r := newTestRegistry()

	got := r.Get("not.exist", "fallback value")
	assert.Equal(t, "fallback value", got)
}

func TestRegistry_Get_EmptyFallback(t *testing.T) {
	r := newTestRegistry()

	got := r.Get("not.exist", "")
	assert.Equal(t, "", got)
}

func TestRegistry_Render_WithVars(t *testing.T) {
	r := newTestRegistry()
	r.cache["tmpl.key"] = "Hello, {{.Name}}!"

	got, err := r.Render("tmpl.key", map[string]any{"Name": "World"}, "fallback")
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", got)
}

func TestRegistry_Render_Fallback(t *testing.T) {
	r := newTestRegistry()

	// key 不存在，用 fallback 渲染
	got, err := r.Render("not.exist", map[string]any{"Name": "Go"}, "Hi, {{.Name}}!")
	require.NoError(t, err)
	assert.Equal(t, "Hi, Go!", got)
}

func TestRegistry_Keys(t *testing.T) {
	r := newTestRegistry()
	r.cache["a.b"] = "1"
	r.cache["c.d"] = "2"
	r.cache["e.f"] = "3"

	keys := r.Keys()
	sort.Strings(keys)
	assert.Equal(t, []string{"a.b", "c.d", "e.f"}, keys)
}

func TestRegistry_ConcurrentGet(t *testing.T) {
	r := newTestRegistry()
	r.cache["key"] = "value"

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			got := r.Get("key", "fallback")
			assert.Equal(t, "value", got)
		}()
	}
	wg.Wait()
}

func TestRegistry_Integration_LoadAndGet(t *testing.T) {
	dir := t.TempDir()
	promptsDir := filepath.Join(dir, "prompts")
	subDir := filepath.Join(promptsDir, "temu")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	yamlContent := `attribute_mapping:
  system:
    content: "你是属性映射专家"
  user:
    content: "{{.Title}}"
`
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "attr.yaml"), []byte(yamlContent), 0644))

	r := newTestRegistry()
	require.NoError(t, r.Init(context.Background(), promptsDir, false))

	assert.Equal(t, "你是属性映射专家", r.Get("temu.attribute_mapping.system", ""))
	assert.Equal(t, "{{.Title}}", r.Get("temu.attribute_mapping.user", ""))
	assert.Equal(t, "fallback", r.Get("not.exist", "fallback"))
}

func TestRegistry_Integration_HotReload(t *testing.T) {
	// 文件放在 promptsDir 直接子目录下，buildFilePrefix 找到 "prompts/" 标记后前缀为空
	// key 格式为 "greeting"；watcher 监听 promptsDir，文件在其中可触发事件
	base := t.TempDir()
	promptsDir := filepath.Join(base, "prompts")
	require.NoError(t, os.MkdirAll(promptsDir, 0755))

	yamlFile := filepath.Join(promptsDir, "msg.yaml")
	initialContent := `greeting:
  content: "old content"
`
	require.NoError(t, os.WriteFile(yamlFile, []byte(initialContent), 0644))

	r := newTestRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, r.Init(ctx, promptsDir, true))
	assert.Equal(t, "old content", r.Get("greeting", ""), "初始加载应读到 old content")

	// 修改文件内容
	updatedContent := `greeting:
  content: "new content"
`
	require.NoError(t, os.WriteFile(yamlFile, []byte(updatedContent), 0644))

	// 轮询等待热更新生效（最多 3 秒，每 100ms 检查一次）
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.Get("greeting", "") == "new content" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.Equal(t, "new content", r.Get("greeting", ""), "热更新应在 3 秒内生效")
}
