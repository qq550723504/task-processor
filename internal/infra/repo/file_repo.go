// Package repo 提供数据操作层
package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"task-processor/internal/domain/model"
)

// FileRepository 文件操作仓储
type FileRepository struct{}

// NewFileRepository 创建文件仓储实例
func NewFileRepository() *FileRepository {
	return &FileRepository{}
}

// SaveProduct 将产品信息保存到文件
func (r *FileRepository) SaveProduct(product *model.Product, filename string) error {
	// 创建文件
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// 创建产品数组（与示例文件格式匹配）
	products := []model.Product{*product}

	// 序列化为JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(products); err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	return nil
}

// SaveProducts 将多个产品信息保存到文件
func (r *FileRepository) SaveProducts(products []model.Product, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(products); err != nil {
		return fmt.Errorf("序列化JSON失败: %w", err)
	}

	return nil
}
