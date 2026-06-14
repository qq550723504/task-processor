package shein

import (
	sheinmarketplace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildInspection(pkg *sheinpub.Package) *sheinpub.Inspection {
	return sheinmarketplace.BuildInspection(pkg)
}
