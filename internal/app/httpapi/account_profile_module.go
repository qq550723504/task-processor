package httpapi

import (
	"gorm.io/gorm"
	profileStore "task-processor/internal/integration/persistence/accountprofile"
	kernelmodule "task-processor/internal/kernel/module"
)

func buildAccountProfileModule(db *gorm.DB) (kernelmodule.Module, error) {
	repository, err := profileStore.New(db)
	if err != nil {
		return nil, err
	}
	return accountProfileModule{repository: repository}, nil
}
