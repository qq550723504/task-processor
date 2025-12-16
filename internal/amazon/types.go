// Package amazon 提供Amazon产品类型常量定义
package amazon

// ProductType 产品类型常量
const (
	// 服装类
	ProductTypeApparel = "APPAREL"
	ProductTypeShirt   = "SHIRT"
	ProductTypeDress   = "DRESS"
	ProductTypePants   = "PANTS"
	ProductTypeShoes   = "SHOES"

	// 电子产品类
	ProductTypeElectronics = "ELECTRONICS"
	ProductTypeCellPhone   = "CELLULAR_PHONE"
	ProductTypeTablet      = "TABLET_COMPUTER"
	ProductTypeLaptop      = "LAPTOP_COMPUTER"
	ProductTypeHeadphones  = "HEADPHONES"
	ProductTypeSmartWatch  = "SMART_WATCH"
	ProductTypeCamera      = "CAMERA"
	ProductTypeTelevision  = "TELEVISION"
	ProductTypeGameConsole = "GAME_CONSOLE"
	ProductTypeVideoGame   = "VIDEO_GAME"

	// 家居用品类
	ProductTypeHome       = "HOME"
	ProductTypeKitchen    = "KITCHEN"
	ProductTypeFurniture  = "FURNITURE"
	ProductTypeBedding    = "BEDDING"
	ProductTypeDecoration = "HOME_DECORATION"
	ProductTypeLighting   = "LIGHTING"
	ProductTypeStorage    = "STORAGE_ORGANIZATION"

	// 美容健康类
	ProductTypeBeauty    = "BEAUTY"
	ProductTypeSkincare  = "SKIN_CARE"
	ProductTypeMakeup    = "MAKEUP"
	ProductTypeFragrance = "FRAGRANCE"
	ProductTypeHairCare  = "HAIR_CARE"
	ProductTypeHealth    = "HEALTH_PERSONAL_CARE"
	ProductTypeVitamins  = "VITAMINS_DIETARY_SUPPLEMENTS"

	// 运动户外类
	ProductTypeSports        = "SPORTS"
	ProductTypeOutdoor       = "OUTDOOR_RECREATION"
	ProductTypeFitness       = "FITNESS"
	ProductTypeSportingGoods = "SPORTING_GOODS"
	ProductTypeExercise      = "EXERCISE_FITNESS"

	// 汽车用品类
	ProductTypeAutomotive     = "AUTOMOTIVE"
	ProductTypeCarAccessory   = "CAR_ACCESSORY"
	ProductTypeMotorcycle     = "MOTORCYCLE"
	ProductTypeTires          = "TIRES"
	ProductTypeCarElectronics = "CAR_ELECTRONICS"

	// 工具五金类
	ProductTypeTools      = "TOOLS"
	ProductTypeHardware   = "HARDWARE"
	ProductTypeGarden     = "GARDEN"
	ProductTypePowerTools = "POWER_TOOLS"
	ProductTypeHandTools  = "HAND_TOOLS"

	// 玩具游戏类
	ProductTypeToys        = "TOYS"
	ProductTypeGames       = "GAMES"
	ProductTypeBabyToys    = "BABY_TOYS"
	ProductTypeEducational = "EDUCATIONAL_TOYS"
	ProductTypeOutdoorToys = "OUTDOOR_TOYS"

	// 书籍媒体类
	ProductTypeBooks    = "BOOKS"
	ProductTypeMusic    = "MUSIC"
	ProductTypeMovies   = "MOVIES"
	ProductTypeSoftware = "SOFTWARE"

	// 办公用品类
	ProductTypeOffice         = "OFFICE_PRODUCTS"
	ProductTypeStationery     = "STATIONERY"
	ProductTypePrinter        = "PRINTER"
	ProductTypeOfficeSupplies = "OFFICE_SUPPLIES"

	// 宠物用品类
	ProductTypePet         = "PET_SUPPLIES"
	ProductTypeDogSupplies = "DOG_SUPPLIES"
	ProductTypeCatSupplies = "CAT_SUPPLIES"
	ProductTypePetFood     = "PET_FOOD"

	// 行李箱包类
	ProductTypeLuggage  = "LUGGAGE"
	ProductTypeBags     = "BAGS"
	ProductTypeBackpack = "BACKPACK"
	ProductTypeHandbag  = "HANDBAG"

	// 婴儿用品类
	ProductTypeBaby          = "BABY_PRODUCT"
	ProductTypeBabyClothing  = "BABY_CLOTHING"
	ProductTypeBabyFood      = "BABY_FOOD"
	ProductTypeBabyFurniture = "BABY_FURNITURE"

	// 食品饮料类
	ProductTypeGrocery  = "GROCERY"
	ProductTypeFood     = "FOOD"
	ProductTypeBeverage = "BEVERAGE"
	ProductTypeSnacks   = "SNACKS"
)

// ProductCategory 产品大类
type ProductCategory struct {
	Name        string
	Types       []string
	Description string
}

// GetProductCategories 获取所有产品大类
func GetProductCategories() map[string]ProductCategory {
	return map[string]ProductCategory{
		"apparel": {
			Name:        "服装鞋帽",
			Types:       []string{ProductTypeApparel, ProductTypeShirt, ProductTypeDress, ProductTypePants, ProductTypeShoes},
			Description: "服装、鞋类、配饰等时尚用品",
		},
		"electronics": {
			Name:        "电子产品",
			Types:       []string{ProductTypeElectronics, ProductTypeCellPhone, ProductTypeTablet, ProductTypeLaptop, ProductTypeHeadphones, ProductTypeSmartWatch, ProductTypeCamera, ProductTypeTelevision, ProductTypeGameConsole, ProductTypeVideoGame},
			Description: "手机、电脑、相机、游戏设备等电子产品",
		},
		"home": {
			Name:        "家居用品",
			Types:       []string{ProductTypeHome, ProductTypeKitchen, ProductTypeFurniture, ProductTypeBedding, ProductTypeDecoration, ProductTypeLighting, ProductTypeStorage},
			Description: "家具、厨具、装饰品等家居用品",
		},
		"beauty": {
			Name:        "美容健康",
			Types:       []string{ProductTypeBeauty, ProductTypeSkincare, ProductTypeMakeup, ProductTypeFragrance, ProductTypeHairCare, ProductTypeHealth, ProductTypeVitamins},
			Description: "化妆品、护肤品、保健品等美容健康产品",
		},
		"sports": {
			Name:        "运动户外",
			Types:       []string{ProductTypeSports, ProductTypeOutdoor, ProductTypeFitness, ProductTypeSportingGoods, ProductTypeExercise},
			Description: "运动器材、户外用品、健身设备等",
		},
		"automotive": {
			Name:        "汽车用品",
			Types:       []string{ProductTypeAutomotive, ProductTypeCarAccessory, ProductTypeMotorcycle, ProductTypeTires, ProductTypeCarElectronics},
			Description: "汽车配件、摩托车用品、轮胎等",
		},
		"tools": {
			Name:        "工具五金",
			Types:       []string{ProductTypeTools, ProductTypeHardware, ProductTypeGarden, ProductTypePowerTools, ProductTypeHandTools},
			Description: "工具、五金、园艺用品等",
		},
		"toys": {
			Name:        "玩具游戏",
			Types:       []string{ProductTypeToys, ProductTypeGames, ProductTypeBabyToys, ProductTypeEducational, ProductTypeOutdoorToys},
			Description: "玩具、游戏、教育用品等",
		},
		"media": {
			Name:        "书籍媒体",
			Types:       []string{ProductTypeBooks, ProductTypeMusic, ProductTypeMovies, ProductTypeSoftware},
			Description: "书籍、音乐、电影、软件等媒体产品",
		},
		"office": {
			Name:        "办公用品",
			Types:       []string{ProductTypeOffice, ProductTypeStationery, ProductTypePrinter, ProductTypeOfficeSupplies},
			Description: "办公设备、文具、打印机等办公用品",
		},
		"pet": {
			Name:        "宠物用品",
			Types:       []string{ProductTypePet, ProductTypeDogSupplies, ProductTypeCatSupplies, ProductTypePetFood},
			Description: "宠物食品、用品、玩具等",
		},
		"luggage": {
			Name:        "箱包配饰",
			Types:       []string{ProductTypeLuggage, ProductTypeBags, ProductTypeBackpack, ProductTypeHandbag},
			Description: "行李箱、背包、手提包等箱包产品",
		},
		"baby": {
			Name:        "婴儿用品",
			Types:       []string{ProductTypeBaby, ProductTypeBabyClothing, ProductTypeBabyFood, ProductTypeBabyFurniture},
			Description: "婴儿服装、食品、家具等婴儿用品",
		},
		"grocery": {
			Name:        "食品饮料",
			Types:       []string{ProductTypeGrocery, ProductTypeFood, ProductTypeBeverage, ProductTypeSnacks},
			Description: "食品、饮料、零食等消费品",
		},
	}
}
