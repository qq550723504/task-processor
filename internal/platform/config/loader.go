package config

// ConfigLoader defines the lifecycle of a configuration loader.
type ConfigLoader interface {
	Load(path string) error
	Reload() error
	Validate() error
	GetConfig() any
}

// ConfigValidator validates decoded configuration values.
type ConfigValidator interface {
	Validate(config any) error
}

// BaseConfigLoader provides path storage and optional validation for loaders.
type BaseConfigLoader struct {
	configPath string
	validator  ConfigValidator
}

// NewBaseConfigLoader creates a base configuration loader.
func NewBaseConfigLoader(path string, validator ConfigValidator) *BaseConfigLoader {
	return &BaseConfigLoader{configPath: path, validator: validator}
}

// GetConfigPath returns the configured path.
func (b *BaseConfigLoader) GetConfigPath() string {
	return b.configPath
}

// ValidateConfig validates config when a validator is configured.
func (b *BaseConfigLoader) ValidateConfig(config any) error {
	if b.validator != nil {
		return b.validator.Validate(config)
	}
	return nil
}
