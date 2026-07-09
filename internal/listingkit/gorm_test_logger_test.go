package listingkit

import "gorm.io/gorm/logger"

func init() {
	logger.Default = logger.Default.LogMode(logger.Silent)
}
