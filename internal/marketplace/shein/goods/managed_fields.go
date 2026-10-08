package goods

import (
	"fmt"
	"strconv"
	model "task-processor/internal/marketplace/shein/model"
	"time"
)

func (b *officialBuild) managedFields() {
	for i, skc := range b.result.Product.SKCs {
		path := fmt.Sprintf("skc_list.%d", i)
		managed := b.rules.ApplicationMode == model.ModeSemiManaged || b.rules.ApplicationMode == model.ModeFullyManaged
		if (managed || skc.ShelfWay != "") && skc.ShelfWay != "1" && skc.ShelfWay != "2" {
			b.issue(path+".shelf_way", "missing", "选择自动上架或定时上架")
		}
		if (b.rules.ApplicationMode == model.ModeFullyManaged || skc.ShelfRequire != "") && skc.ShelfRequire != "0" && skc.ShelfRequire != "1" {
			b.issue(path+".shelf_require", "missing", "选择是否到货 SHEIN 仓后上架")
		}
		if skc.ShelfWay == "2" {
			parsed, err := time.Parse("2006-01-02 15:04:05", skc.HopeOnSaleDate)
			if err != nil || parsed.Format("2006-01-02 15:04:05") != skc.HopeOnSaleDate {
				b.issue(path+".hope_on_sale_date", "invalid", "填写规范的北京时间上架时间：年-月-日 时:分:秒")
			}
		} else if skc.HopeOnSaleDate != "" {
			b.issue(path+".hope_on_sale_date", "invalid", "自动上架不填写定时上架时间")
		}
	}
	sample := b.result.Product.Sample
	if sample == nil {
		return
	}
	if sample.JudgeType != 2 || sample.ReserveFlag != 2 || sample.SpotFlag != 1 && sample.SpotFlag != 2 {
		b.issue("sample_info", "invalid", "样品固定为大货样衣、不留样，并明确是否现货")
	}
	if len(sample.Spec.Sub) > 2 {
		b.issue("sample_info.sample_spec", "invalid", "样品规格最多包含两项 SKU 销售属性")
		return
	}
	match := false
	for _, skc := range b.result.Product.SKCs {
		if skc.SaleAttribute.AttributeID != sample.Spec.Main.AttributeID || skc.SaleAttribute.AttributeValueID == nil || *skc.SaleAttribute.AttributeValueID != sample.Spec.Main.ValueID {
			continue
		}
		for _, sku := range skc.SKUs {
			if len(sample.Spec.Sub) != len(sku.SaleAttributes) {
				continue
			}
			valid := true
			seen := map[int64]bool{}
			for _, sub := range sample.Spec.Sub {
				id, ie := strconv.ParseInt(sub.AttributeID, 10, 64)
				value, ve := strconv.ParseInt(sub.ValueID, 10, 64)
				found := false
				for _, sale := range sku.SaleAttributes {
					found = found || sale.AttributeID == id && sale.AttributeValueID != nil && *sale.AttributeValueID == value
				}
				if ie != nil || ve != nil || id <= 0 || value <= 0 || seen[id] || strconv.FormatInt(id, 10) != sub.AttributeID || strconv.FormatInt(value, 10) != sub.ValueID || !found {
					valid = false
				}
				seen[id] = true
			}
			match = match || valid
		}
	}
	if !match {
		b.issue("sample_info.sample_spec", "invalid", "样品规格须精确匹配本商品实际 SKC 和 SKU 销售规格")
	}
}
