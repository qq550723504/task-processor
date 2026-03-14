// Package repository 提供数据操作层实现
package repository

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
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	products := []model.Product{*product}

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
