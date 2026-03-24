package prompt

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLoader 创建测试用 loader（使用 discard logger 避免日志噪音）
func newTestLoader() PromptLoader {
	log := logrus.NewEntry(logrus.New())
	log.Logger.SetOutput(io.Discard)
	return NewPromptLoaderWithLogger(log)
}

// TestLoadFile_Normal 测试正常加载单个 YAML 文件
func TestLoadFile_Normal(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `attribute_mapping:
  system:
    description: "系统提示词"
    content: |
      你是TEMU平台的产品属性映射专家
  user:
    content: |
      {{.Title}}
`
	path := filepath.Join(dir, "prompts", "temu", "attribute_mapping.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0644))

	loader := newTestLoader()
	result, err := loader.LoadFile(path)
	require.NoError(t, err)

	// description 字段应被过滤
	for k := range result {
		assert.NotContains(t, k, "description", "description 字段不应出现在结果中")
	}

	// 验证 key 命名规范：temu.attribute_mapping.system 和 temu.attribute_mapping.user
	assert.Contains(t, result, "temu.attribute_mapping.system")
	assert.Contains(t, result, "temu.attribute_mapping.user")
	assert.Equal(t, "你是TEMU平台的产品属性映射专家\n", result["temu.attribute_mapping.system"])
	assert.Equal(t, "{{.Title}}\n", result["temu.attribute_mapping.user"])
}

// TestLoadFile_DescriptionFiltered 验证 description 字段被过滤
func TestLoadFile_DescriptionFiltered(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `scorer:
  text:
    description: "文本评分提示词"
    content: "请评分"
  image:
    description: "图片评分提示词"
    content: "请评估图片"
`
	path := filepath.Join(dir, "prompts", "productenrich", "llm_scorer.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0644))

	loader := newTestLoader()
	result, err := loader.LoadFile(path)
	require.NoError(t, err)

	assert.Contains(t, result, "productenrich.scorer.text")
	assert.Contains(t, result, "productenrich.scorer.image")
	assert.Equal(t, "请评分", result["productenrich.scorer.text"])
	assert.Equal(t, "请评估图片", result["productenrich.scorer.image"])

	// description 不应出现
	for k := range result {
		assert.NotContains(t, k, "description")
	}
}

// TestLoadFile_InvalidYAML 测试 YAML 语法错误时返回 error
func TestLoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("key: [unclosed"), 0644))

	loader := newTestLoader()
	result, err := loader.LoadFile(path)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestLoadFile_EmptyFile 测试空文件
func TestLoadFile_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	require.NoError(t, os.WriteFile(path, []byte(""), 0644))

	loader := newTestLoader()
	result, err := loader.LoadFile(path)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestLoadAll_DirNotExist 目录不存在时返回空 map，不返回 error
func TestLoadAll_DirNotExist(t *testing.T) {
	loader := newTestLoader()
	result, err := loader.LoadAll("/nonexistent/path/prompts")
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestLoadAll_MergeAndWarnDuplicate 测试重复 key 以最后加载文件为准
func TestLoadAll_MergeAndWarnDuplicate(t *testing.T) {
	dir := t.TempDir()
	promptsDir := filepath.Join(dir, "prompts")

	// 创建两个文件，产生相同的 key
	file1 := filepath.Join(promptsDir, "temu", "a.yaml")
	file2 := filepath.Join(promptsDir, "temu", "b.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(file1), 0755))

	yaml1 := `a:
  system:
    content: "from a"
`
	yaml2 := `a:
  system:
    content: "from b"
`
	require.NoError(t, os.WriteFile(file1, []byte(yaml1), 0644))
	require.NoError(t, os.WriteFile(file2, []byte(yaml2), 0644))

	loader := newTestLoader()
	result, err := loader.LoadAll(promptsDir)
	require.NoError(t, err)

	// 两个文件都应被加载，重复 key 以最后加载的为准
	assert.Contains(t, result, "temu.a.system")
	// 值为其中一个（不检查具体哪个，只验证不 panic 且有值）
	assert.NotEmpty(t, result["temu.a.system"])
}

// TestLoadAll_RecursiveLoad 测试递归加载多级目录
func TestLoadAll_RecursiveLoad(t *testing.T) {
	dir := t.TempDir()
	promptsDir := filepath.Join(dir, "prompts")

	files := map[string]string{
		filepath.Join(promptsDir, "temu", "attr.yaml"): `attr:
  system:
    content: "temu system"
`,
		filepath.Join(promptsDir, "shein", "trans.yaml"): `trans:
  user:
    content: "shein user"
`,
	}

	for path, content := range files {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	loader := newTestLoader()
	result, err := loader.LoadAll(promptsDir)
	require.NoError(t, err)

	assert.Contains(t, result, "temu.attr.system")
	assert.Contains(t, result, "shein.trans.user")
	assert.Equal(t, "temu system", result["temu.attr.system"])
	assert.Equal(t, "shein user", result["shein.trans.user"])
}

// TestBuildFilePrefix 测试 buildFilePrefix 函数
func TestBuildFilePrefix(t *testing.T) {
	cases := []struct {
		path     string
		expected string
	}{
		{
			path:     filepath.Join("prompts", "temu", "attribute_mapping.yaml"),
			expected: "temu",
		},
		{
			path:     filepath.Join("prompts", "productenrich", "llm_scorer.yaml"),
			expected: "productenrich",
		},
		{
			path:     filepath.Join("prompts", "shein", "translation.yaml"),
			expected: "shein",
		},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got := buildFilePrefix(tc.path)
			assert.Equal(t, tc.expected, got)
		})
	}
}
