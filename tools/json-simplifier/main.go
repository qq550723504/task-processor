package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	var (
		inputFile  string
		outputFile string
		maxLen     int
	)

	flag.StringVar(&inputFile, "i", "", "输入JSON文件路径 (留空则从stdin读取)")
	flag.StringVar(&outputFile, "o", "", "输出文件路径 (留空则输出到stdout)")
	flag.IntVar(&maxLen, "max", 10, "字符串最大长度")
	flag.Parse()

	// 读取输入
	var input []byte
	var err error
	if inputFile != "" {
		input, err = os.ReadFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取文件失败: %v\n", err)
			os.Exit(1)
		}
	} else {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "读取stdin失败: %v\n", err)
			os.Exit(1)
		}
	}

	// 解析JSON
	var data any
	if unmarshalErr := json.Unmarshal(input, &data); unmarshalErr != nil {
		fmt.Fprintf(os.Stderr, "JSON解析失败: %v\n", unmarshalErr)
		os.Exit(1)
	}

	// 简化数据
	simplified := simplify(data, maxLen)

	// 输出结果
	output, err := json.MarshalIndent(simplified, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON生成失败: %v\n", err)
		os.Exit(1)
	}

	if outputFile != "" {
		if err := os.WriteFile(outputFile, output, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "写入文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("已保存到: %s\n", outputFile)
	} else {
		fmt.Println(string(output))
	}
}

func simplify(data any, maxLen int) any {
	switch v := data.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			result[key] = simplify(val, maxLen)
		}
		return result

	case []any:
		if len(v) == 0 {
			return []any{}
		}
		// 只保留第一个元素作为示例
		return []any{simplify(v[0], maxLen)}

	case string:
		return simplifyString(v, maxLen)

	case float64, int, bool, nil:
		return v

	default:
		return v
	}
}

func simplifyString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// 检查是否是特殊格式
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return "https://..."
	}
	if strings.Contains(s, "@") && strings.Contains(s, ".") {
		return "email@example.com"
	}
	if len(s) > 20 && !strings.Contains(s, " ") {
		// 可能是ID或token
		return "id_or_token"
	}

	// 普通字符串截断
	return s[:maxLen] + "..."
}
