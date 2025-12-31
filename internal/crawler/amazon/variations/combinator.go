package variations

// Combinator 组合生成器
type Combinator struct {
	config *Config
}

// NewCombinator 创建组合生成器
func NewCombinator(config *Config) *Combinator {
	return &Combinator{config: config}
}

// Generate 生成所有属性组合
func (c *Combinator) Generate(dimensions map[string][]string) []map[string]interface{} {
	var combinations []map[string]interface{}

	// 获取维度名称和值
	var dimNames []string
	var dimValues [][]string

	for name, values := range dimensions {
		dimNames = append(dimNames, name)
		dimValues = append(dimValues, values)
	}

	if len(dimNames) == 0 {
		return combinations
	}

	// 递归生成组合
	c.generateRecursive(dimNames, dimValues, 0, make(map[string]interface{}), &combinations)

	return combinations
}

// generateRecursive 递归生成组合
func (c *Combinator) generateRecursive(
	dimNames []string,
	dimValues [][]string,
	index int,
	current map[string]interface{},
	combinations *[]map[string]interface{},
) {
	if index == len(dimNames) {
		// 复制当前组合
		combo := make(map[string]interface{})
		for k, v := range current {
			combo[k] = v
		}
		*combinations = append(*combinations, combo)
		return
	}

	// 遍历当前维度的所有值
	for _, value := range dimValues[index] {
		current[dimNames[index]] = value
		c.generateRecursive(dimNames, dimValues, index+1, current, combinations)
	}
}
