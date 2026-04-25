package platforms

import (
	"task-processor/internal/app/consumer"
	platformamazon "task-processor/internal/platforms/amazon"
	"task-processor/internal/platforms/shein"
	"task-processor/internal/platforms/temu"
)

func All() []consumer.PlatformModule {
	return []consumer.PlatformModule{
		platformamazon.NewModule(),
		temu.NewModule(),
		shein.NewModule(),
	}
}
