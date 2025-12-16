// Package model 提供Amazon产品类型常量定义
package model

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

	// 美容护理类
	ProductTypeBeauty    = "BEAUTY"
	ProductTypeSkincare  = "SKINCARE"
	ProductTypeMakeup    = "MAKEUP"
	ProductTypeFragrance = "FRAGRANCE"
	ProductTypeHaircare  = "HAIRCARE"

	// 运动户外类
	ProductTypeSports   = "SPORTS"
	ProductTypeOutdoor  = "OUTDOOR"
	ProductTypeFitness  = "FITNESS"
	ProductTypeCycling  = "CYCLING"
	ProductTypeSwimming = "SWIMMING"

	// 汽车用品类
	ProductTypeAutomotive = "AUTOMOTIVE"
	ProductTypeCarParts   = "CAR_PARTS"
	ProductTypeCarCare    = "CAR_CARE"
	ProductTypeMotorcycle = "MOTORCYCLE"

	// 书籍媒体类
	ProductTypeBooks    = "BOOKS"
	ProductTypeMusic    = "MUSIC"
	ProductTypeMovies   = "MOVIES"
	ProductTypeSoftware = "SOFTWARE"

	// 玩具游戏类
	ProductTypeToys        = "TOYS"
	ProductTypeGames       = "GAMES"
	ProductTypePuzzles     = "PUZZLES"
	ProductTypeEducational = "EDUCATIONAL_TOYS"

	// 健康个护类
	ProductTypeHealth       = "HEALTH"
	ProductTypePersonalCare = "PERSONAL_CARE"
	ProductTypeVitamins     = "VITAMINS"
	ProductTypeMedical      = "MEDICAL_SUPPLIES"

	// 宠物用品类
	ProductTypePet     = "PET"
	ProductTypePetFood = "PET_FOOD"
	ProductTypePetToys = "PET_TOYS"
	ProductTypePetCare = "PET_CARE"

	// 办公用品类
	ProductTypeOffice     = "OFFICE"
	ProductTypeStationery = "STATIONERY"
	ProductTypePrinting   = "PRINTING"
	ProductTypeStorage    = "STORAGE"

	// 工具五金类
	ProductTypeTools        = "TOOLS"
	ProductTypeHardware     = "HARDWARE"
	ProductTypeGarden       = "GARDEN"
	ProductTypeConstruction = "CONSTRUCTION"

	// 食品饮料类
	ProductTypeFood     = "FOOD"
	ProductTypeBeverage = "BEVERAGE"
	ProductTypeSnacks   = "SNACKS"
	ProductTypeOrganic  = "ORGANIC_FOOD"

	// 婴幼儿用品类
	ProductTypeBaby     = "BABY"
	ProductTypeBabyFood = "BABY_FOOD"
	ProductTypeBabyToys = "BABY_TOYS"
	ProductTypeBabyCare = "BABY_CARE"

	// 乐器类
	ProductTypeMusicalInstruments = "MUSICAL_INSTRUMENTS"
	ProductTypeGuitar             = "GUITAR"
	ProductTypePiano              = "PIANO"
	ProductTypeDrums              = "DRUMS"

	// 珠宝首饰类
	ProductTypeJewelry     = "JEWELRY"
	ProductTypeWatches     = "WATCHES"
	ProductTypeAccessories = "ACCESSORIES"

	// 行李箱包类
	ProductTypeLuggage   = "LUGGAGE"
	ProductTypeBags      = "BAGS"
	ProductTypeBackpacks = "BACKPACKS"
	ProductTypeWallets   = "WALLETS"
)

// ProductTypeCategories 产品类型分类映射
var ProductTypeCategories = map[string][]string{
	"服装鞋帽": {
		ProductTypeApparel, ProductTypeShirt, ProductTypeDress,
		ProductTypePants, ProductTypeShoes,
	},
	"电子产品": {
		ProductTypeElectronics, ProductTypeCellPhone, ProductTypeTablet,
		ProductTypeLaptop, ProductTypeHeadphones, ProductTypeSmartWatch,
		ProductTypeCamera, ProductTypeTelevision, ProductTypeGameConsole,
		ProductTypeVideoGame,
	},
	"家居用品": {
		ProductTypeHome, ProductTypeKitchen, ProductTypeFurniture,
		ProductTypeBedding, ProductTypeDecoration,
	},
	"美容护理": {
		ProductTypeBeauty, ProductTypeSkincare, ProductTypeMakeup,
		ProductTypeFragrance, ProductTypeHaircare,
	},
	"运动户外": {
		ProductTypeSports, ProductTypeOutdoor, ProductTypeFitness,
		ProductTypeCycling, ProductTypeSwimming,
	},
	"汽车用品": {
		ProductTypeAutomotive, ProductTypeCarParts, ProductTypeCarCare,
		ProductTypeMotorcycle,
	},
	"书籍媒体": {
		ProductTypeBooks, ProductTypeMusic, ProductTypeMovies,
		ProductTypeSoftware,
	},
	"玩具游戏": {
		ProductTypeToys, ProductTypeGames, ProductTypePuzzles,
		ProductTypeEducational,
	},
	"健康个护": {
		ProductTypeHealth, ProductTypePersonalCare, ProductTypeVitamins,
		ProductTypeMedical,
	},
	"宠物用品": {
		ProductTypePet, ProductTypePetFood, ProductTypePetToys,
		ProductTypePetCare,
	},
}

// GetCategoryByProductType 根据产品类型获取分类
func GetCategoryByProductType(productType string) string {
	for category, types := range ProductTypeCategories {
		for _, t := range types {
			if t == productType {
				return category
			}
		}
	}
	return "其他"
}

// IsValidProductType 检查产品类型是否有效
func IsValidProductType(productType string) bool {
	for _, types := range ProductTypeCategories {
		for _, t := range types {
			if t == productType {
				return true
			}
		}
	}
	return false
}

// GetProductTypesByCategory 根据分类获取产品类型列表
func GetProductTypesByCategory(category string) []string {
	if types, exists := ProductTypeCategories[category]; exists {
		return types
	}
	return []string{}
}

// GetAllCategories 获取所有分类
func GetAllCategories() []string {
	categories := make([]string, 0, len(ProductTypeCategories))
	for category := range ProductTypeCategories {
		categories = append(categories, category)
	}
	return categories
}

// GetAllProductTypes 获取所有产品类型
func GetAllProductTypes() []string {
	var allTypes []string
	for _, types := range ProductTypeCategories {
		allTypes = append(allTypes, types...)
	}
	return allTypes
}
