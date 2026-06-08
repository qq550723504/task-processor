package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=127.0.0.1 port=15432 user=postgres password=123456 dbname=ruoyi-vue-pro sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect database: %v", err)
	}

	// 第一步：查询需要修复的任务数量
	var count int64
	result := db.Raw(`
		SELECT COUNT(*) 
		FROM listing_kit_tasks 
		WHERE status = 'completed' 
		  AND result::jsonb ? 'pod_execution'
		  AND (result::jsonb->'pod_execution')->>'status' = 'succeeded'
		  AND NOT (result::jsonb ? 'shein')
		  AND created_at > NOW() - INTERVAL '2 hours'
	`).Scan(&count)

	if result.Error != nil {
		log.Fatalf("查询失败: %v", result.Error)
	}

	fmt.Printf("=== 找到 %d 个需要修复的任务 ===\n\n", count)

	if count == 0 {
		fmt.Println("没有需要修复的任务")
		return
	}

	// 第二步：执行更新
	fmt.Println("正在将任务状态改回 pending...")
	updateResult := db.Exec(`
		UPDATE listing_kit_tasks 
		SET status = 'pending', 
		    error = '', 
		    updated_at = NOW()
		WHERE status = 'completed' 
		  AND result::jsonb ? 'pod_execution'
		  AND (result::jsonb->'pod_execution')->>'status' = 'succeeded'
		  AND NOT (result::jsonb ? 'shein')
		  AND created_at > NOW() - INTERVAL '2 hours'
	`)

	if updateResult.Error != nil {
		log.Fatalf("更新失败: %v", updateResult.Error)
	}

	fmt.Printf("✅ 成功更新 %d 个任务\n", updateResult.RowsAffected)
	fmt.Println("\n这些任务将被 Temporal Worker 自动重新处理")
	fmt.Println("请等待几分钟，然后检查任务状态是否变为 completed 且有 shein 字段")
}
