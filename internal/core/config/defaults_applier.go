// Package config 提供配置默认值应用功能
package config

import (
	"reflect"
)

// DefaultsApplier 默认值应用器
type DefaultsApplier struct {
	defaultConfig *Config
}

// NewDefaultsApplier 创建默认值应用器
func NewDefaultsApplier() *DefaultsApplier {
	return &DefaultsApplier{
		defaultConfig: NewDefaultConfig(),
	}
}

// Apply 应用默认值到配置
func (da *DefaultsApplier) Apply(cfg *Config) {
	da.applyStructDefaults(reflect.ValueOf(cfg).Elem(), reflect.ValueOf(da.defaultConfig).Elem())
}

// applyStructDefaults 递归应用结构体默认值
func (da *DefaultsApplier) applyStructDefaults(target, source reflect.Value) {
	if !target.IsValid() || !source.IsValid() {
		return
	}

	for i := 0; i < target.NumField(); i++ {
		targetField := target.Field(i)
		sourceField := source.Field(i)

		// 跳过不可设置的字段
		if !targetField.CanSet() {
			continue
		}

		// 根据字段类型处理
		switch targetField.Kind() {
		case reflect.Struct:
			// 递归处理嵌套结构体
			da.applyStructDefaults(targetField, sourceField)

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			// 整数类型：如果为0则使用默认值
			if targetField.Int() == 0 {
				targetField.SetInt(sourceField.Int())
			}

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			// 无符号整数类型：如果为0则使用默认值
			if targetField.Uint() == 0 {
				targetField.SetUint(sourceField.Uint())
			}

		case reflect.Float32, reflect.Float64:
			// 浮点数类型：如果为0则使用默认值
			if targetField.Float() == 0 {
				targetField.SetFloat(sourceField.Float())
			}

		case reflect.String:
			// 字符串类型：如果为空则使用默认值
			if targetField.String() == "" {
				targetField.SetString(sourceField.String())
			}

		case reflect.Bool:
			// 布尔类型：不应用默认值（false是有效值）
			// 如果需要特殊处理，可以通过tag标记

		case reflect.Slice:
			// 切片类型：如果为nil或长度为0则使用默认值
			if targetField.IsNil() || targetField.Len() == 0 {
				if !sourceField.IsNil() && sourceField.Len() > 0 {
					targetField.Set(sourceField)
				}
			}

		case reflect.Map:
			// Map类型：如果为nil则使用默认值
			if targetField.IsNil() {
				if !sourceField.IsNil() {
					targetField.Set(sourceField)
				}
			}

		case reflect.Ptr:
			// 指针类型：如果为nil则使用默认值
			if targetField.IsNil() {
				if !sourceField.IsNil() {
					targetField.Set(sourceField)
				}
			}
		}
	}
}
