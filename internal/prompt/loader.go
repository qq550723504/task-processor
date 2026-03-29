package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// PromptLoader 负责从文件系统加载 prompt YAML 文件
type PromptLoader interface {
	// LoadAll 递归加载目录下所有 .yaml 文件，返回展平后的 key→value 映射
	LoadAll(dir string) (map[string]string, error)
	// LoadFile 加载单个 YAML 文件，返回该文件贡献的 key→value 映射
	LoadFile(path string) (map[string]string, error)
}

// promptLoader PromptLoader 的具体实现
type promptLoader struct {
	log *logrus.Entry
}

// NewPromptLoader 创建一个新的 PromptLoader 实例
func NewPromptLoader() PromptLoader {
	return &promptLoader{
		log: logger.GetGlobalLogger("prompt.loader"),
	}
}

// NewPromptLoaderWithLogger 创建一个使用指定 logger 的 PromptLoader 实例
func NewPromptLoaderWithLogger(log *logrus.Entry) PromptLoader {
	return &promptLoader{log: log}
}

// LoadAll 递归加载目录下所有 .yaml 文件，合并所有 key
// 重复 key 以最后加载的文件为准并记录 WARN 日志
// 目录不存在时记录 WARN 日志并返回空 map，不返回 error
func (l *promptLoader) LoadAll(dir string) (map[string]string, error) {
	result := make(map[string]string)

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		l.log.WithField("dir", dir).Warn("prompts 目录不存在，以空缓存启动")
		return result, nil
	}

	// 递归遍历目录下所有 .yaml 文件
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			l.log.WithError(err).WithField("path", path).Error("遍历目录时发生错误")
			return nil // 跳过错误，继续遍历
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".yaml" {
			return nil
		}

		fileResult, err := l.LoadFile(path)
		if err != nil {
			// 单文件解析失败，记录错误并跳过
			l.log.WithError(err).WithField("file", path).Error("加载 YAML 文件失败，跳过该文件")
			return nil
		}

		// 合并 key，重复 key 记录 WARN
		for k, v := range fileResult {
			if _, exists := result[k]; exists {
				l.log.WithFields(logrus.Fields{
					"key":  k,
					"file": path,
				}).Warn("发现重复的 Prompt_Key，将使用最新加载的文件覆盖")
			}
			result[k] = v
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("遍历 prompts 目录失败: %w", err)
	}

	return result, nil
}

// LoadFile 加载单个 YAML 文件，递归展平嵌套结构为 key→value 映射
// 跳过 description 字段，content 字段的值作为 prompt 内容（key 中去掉 content 层）
func (l *promptLoader) LoadFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败 %s: %w", path, err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("解析 YAML 文件失败 %s: %w", path, err)
	}

	// 空文件
	if root.Kind == 0 {
		return make(map[string]string), nil
	}

	// yaml.Unmarshal 返回的根节点是 DocumentNode，其第一个子节点才是实际内容
	var contentNode *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		contentNode = root.Content[0]
	} else {
		contentNode = &root
	}

	// 计算文件相对于 prompts 目录的前缀（仅目录部分，不含文件名）
	prefix := buildFilePrefix(path)

	result := make(map[string]string)
	// 若前缀非空，直接展平；若为空，展平时不加前缀点号
	flattenYAML(contentNode, prefix, result)
	return result, nil
}

// buildFilePrefix 根据文件路径构建 Prompt_Key 前缀
// 规则：取文件路径中 prompts/ 之后的目录部分（不含文件名），将路径分隔符替换为 .
// 例：prompts/temu/attribute_mapping.yaml → 前缀为 "temu"
//
//	YAML 中 attribute_mapping.system.content → key 为 temu.attribute_mapping.system
//
// 若路径中不含 prompts/ 或文件直接在 prompts/ 下，则前缀为空字符串
func buildFilePrefix(path string) string {
	// 统一使用正斜杠
	normalized := filepath.ToSlash(path)

	// 找到 prompts/ 之后的部分
	const marker = "prompts/"
	idx := strings.LastIndex(normalized, marker)
	var rel string
	if idx >= 0 {
		rel = normalized[idx+len(marker):]
	} else {
		// 不含 prompts/，取文件所在目录名（相对路径最后一级目录）
		rel = filepath.Base(filepath.Dir(path))
		if rel == "." {
			return ""
		}
		return rel
	}

	// rel 现在是 "temu/attribute_mapping.yaml"，取目录部分 "temu"
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." {
		return ""
	}
	return strings.ReplaceAll(dir, "/", ".")
}

// flattenYAML 递归展平 YAML 节点为 key→value 映射
//
// 算法：
//   - 若节点为 map：遍历每个 key-child 对
//   - 若 key == "description"：跳过
//   - 若 key == "content"：result[prefix] = child.Value
//   - 否则：递归处理 child，前缀追加 ".key"（prefix 为空时直接用 key）
//   - 若节点为 scalar：result[prefix] = node.Value
func flattenYAML(node *yaml.Node, prefix string, result map[string]string) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.MappingNode:
		// MappingNode 的 Content 是 [key1, val1, key2, val2, ...]
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			key := keyNode.Value

			if key == "description" {
				continue
			}

			var newPrefix string
			if key == "content" {
				// content 字段：值直接存入当前 prefix，不追加 .content
				result[prefix] = valNode.Value
				continue
			}
			if prefix == "" {
				newPrefix = key
			} else {
				newPrefix = prefix + "." + key
			}
			flattenYAML(valNode, newPrefix, result)
		}

	case yaml.ScalarNode:
		if prefix != "" {
			result[prefix] = node.Value
		}

	case yaml.SequenceNode:
		// 序列节点：将各元素拼接为字符串（一般 prompt 不会是数组，但做兼容处理）
		var parts []string
		for _, child := range node.Content {
			parts = append(parts, child.Value)
		}
		if prefix != "" {
			result[prefix] = strings.Join(parts, "\n")
		}

	case yaml.DocumentNode:
		if len(node.Content) > 0 {
			flattenYAML(node.Content[0], prefix, result)
		}
	}
}
