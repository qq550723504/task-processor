package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	// 解析命令行参数
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	dsn := "host=127.0.0.1 port=15432 user=postgres password=123456 dbname=ruoyi-vue-pro sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 禁用日志
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接数据库失败: %v\n", err)
		os.Exit(1)
	}

	table := ""
	where := ""
	fields := "*"
	limit := 100
	outputFormat := "table" // table, json, csv

	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--table":
			i++
			if i < len(os.Args) {
				table = os.Args[i]
			}
		case "--where":
			i++
			if i < len(os.Args) {
				where = os.Args[i]
			}
		case "--fields":
			i++
			if i < len(os.Args) {
				fields = os.Args[i]
			}
		case "--limit":
			i++
			if i < len(os.Args) {
				fmt.Sscanf(os.Args[i], "%d", &limit)
			}
		case "--format":
			i++
			if i < len(os.Args) {
				outputFormat = os.Args[i]
			}
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		}
	}

	if table == "" {
		fmt.Println("错误: 必须指定 --table 参数")
		printUsage()
		os.Exit(1)
	}

	// 构建查询
	query := db.Table(table).Select(fields)
	if where != "" {
		query = query.Where(where)
	}
	query = query.Limit(limit)

	// 执行查询
	var results []map[string]interface{}
	if err := query.Find(&results).Error; err != nil {
		fmt.Fprintf(os.Stderr, "查询失败: %v\n", err)
		os.Exit(1)
	}

	// 输出结果
	switch outputFormat {
	case "json":
		outputJSON(results)
	case "csv":
		outputCSV(results)
	default:
		outputTable(results)
	}

	fmt.Printf("\n共 %d 条记录\n", len(results))
}

func printUsage() {
	fmt.Println(`数据库查询工具

用法:
  db-query --table <表名> [选项]

选项:
  --table <表名>       要查询的表名 (必需)
  --where <条件>       WHERE 条件 (可选)
  --fields <字段列表>  要查询的字段,用逗号分隔 (默认: *)
  --limit <数量>       限制返回记录数 (默认: 100)
  --format <格式>      输出格式: table, json, csv (默认: table)
  --help, -h           显示帮助信息

示例:
  # 查询批次的所有任务ID
  db-query --table shein_studio_sessions --where "id='batch-id'" --fields created_tasks

  # 查询待处理的任务
  db-query --table listing_kit_tasks --where "status='pending'" --fields task_id,status --limit 50

  # 以 JSON 格式输出
  db-query --table listing_kit_tasks --where "status='completed'" --format json

  # 查询所有字段
  db-query --table listing_kit_tasks --where "task_id='xxx'" --fields "*"
`)
}

func outputTable(results []map[string]interface{}) {
	if len(results) == 0 {
		fmt.Println("没有数据")
		return
	}

	// 获取列名
	var columns []string
	for key := range results[0] {
		columns = append(columns, key)
	}

	// 计算每列的最大宽度
	colWidths := make(map[string]int)
	for _, col := range columns {
		colWidths[col] = len(col)
		for _, row := range results {
			val := fmt.Sprintf("%v", row[col])
			if len(val) > colWidths[col] {
				colWidths[col] = len(val)
			}
		}
	}

	// 打印表头
	header := ""
	for _, col := range columns {
		header += fmt.Sprintf("%-*s | ", colWidths[col], col)
	}
	fmt.Println(header)
	fmt.Println(strings.Repeat("-", len(header)))

	// 打印数据行
	for _, row := range results {
		line := ""
		for _, col := range columns {
			val := fmt.Sprintf("%v", row[col])
			line += fmt.Sprintf("%-*s | ", colWidths[col], val)
		}
		fmt.Println(line)
	}
}

func outputJSON(results []map[string]interface{}) {
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON 序列化失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(jsonData))
}

func outputCSV(results []map[string]interface{}) {
	if len(results) == 0 {
		fmt.Println("没有数据")
		return
	}

	// 获取列名
	var columns []string
	for key := range results[0] {
		columns = append(columns, key)
	}

	// 打印表头
	fmt.Println(strings.Join(columns, ","))

	// 打印数据行
	for _, row := range results {
		var values []string
		for _, col := range columns {
			val := fmt.Sprintf("%v", row[col])
			// 如果包含逗号或引号,需要转义
			if strings.Contains(val, ",") || strings.Contains(val, "\"") {
				val = "\"" + strings.ReplaceAll(val, "\"", "\"\"") + "\""
			}
			values = append(values, val)
		}
		fmt.Println(strings.Join(values, ","))
	}
}
