package goods

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
)

const MaxOfficialImageBytes int64 = 3 << 20

func ValidImageObservation(value OfficialImageObservation) bool {
	hash, err := hex.DecodeString(value.ContentHash)
	return err == nil && len(hash) == 32 && value.Bytes > 0 && value.Bytes <= MaxOfficialImageBytes && (value.MediaType == "image/jpeg" || value.MediaType == "image/png") && value.Width > 0 && value.Height > 0
}

func (b *officialBuild) pictures() map[string]bool {
	rules := map[string]bool{}
	for _, rule := range b.rules.Fill.Pictures {
		if rule.Enabled == nil {
			b.issue("image_info", "rule_unavailable", "当前图片规范不完整")
		} else {
			rules[rule.Field] = *rule.Enabled
		}
	}
	return rules
}
func IsOfficialImageReference(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Fragment != "" || parsed.Port() != "" && parsed.Port() != "443" || len(raw) > 2048 {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "shein.com" || strings.HasSuffix(host, ".shein.com") || host == "ltwebstatic.com" || strings.HasSuffix(host, ".ltwebstatic.com")
}
func (b *officialBuild) images(slots []OfficialImageSlot, observations []OfficialImageObservation) bool {
	p := &b.result.Product
	rules := b.pictures()
	_, newScheme := rules["spu_image_detail_show"]
	p.IsSPUPic = newScheme
	if _, ok := rules["sku_image_required"]; !ok {
		b.issue("image_info", "rule_unavailable", "当前类目的 SKU 图片必填规则不可用")
	}
	if newScheme {
		for _, field := range []string{"spu_image_detail_required", "spu_image_detail_single", "spu_image_square_show", "spu_image_square_required", "skc_image_detail_show", "skc_image_detail_required", "skc_image_detail_single", "skc_image_square_show", "skc_image_square_required"} {
			if _, ok := rules[field]; !ok {
				b.issue("image_info", "rule_unavailable", "当前 SPU 图片方案不完整")
			}
		}
	}
	if inventory := b.inventory; inventory.Scope.TenantID == "" || inventory.Scope.ProductKey == "" || inventory.Scope.TargetPlatform != "shein" || inventory.Scope.SourceSnapshotVersion == 0 {
		b.issue("image_info", "approval_missing", "先确认本次商品版本的完整图片选择")
	}
	assets := map[string]asset.ApprovedAsset{}
	for _, approved := range b.inventory.Assets {
		if assets[approved.ID].ID != "" {
			b.issue("image_info", "approval_invalid", "批准图片存在重复身份")
		}
		assets[approved.ID] = approved
	}
	// Direct image URLs are not user-editable target fields. Every image must
	// resolve from the exact Asset inventory through a typed selected slot.
	if p.ImageInfo != nil {
		b.issue("image_info", "unapproved", "请从已批准图片中选择，不接受直接填写图片链接")
	}
	p.ImageInfo = nil
	for i := range p.SKCs {
		skc := &p.SKCs[i]
		if len(skc.ImageInfo.Images) > 0 || len(skc.SiteDetailImages) > 0 {
			b.issue(fmt.Sprintf("skc_list.%d.image_info", i), "unapproved", "请从已批准图片中选择")
		}
		skc.ImageInfo.Images = nil
		skc.SiteDetailImages = nil
		for j := range skc.SKUs {
			if skc.SKUs[j].ImageInfo != nil {
				b.issue(fmt.Sprintf("skc_list.%d.sku_list.%d.image_info", i, j), "unapproved", "请从已批准图片中选择")
			}
			skc.SKUs[j].ImageInfo = nil
		}
	}
	if len(slots) > 10000 {
		b.issue("image_info", "invalid", "图片位置数量超过限制")
		return false
	}
	groups := map[string][]model.ProductImage{}
	sortSeen := map[string]bool{}
	allRemote := len(slots) > 0
	for _, slot := range slots {
		path := "image_info"
		valid := true
		switch slot.Group {
		case "spu":
			valid = newScheme && slot.SKC == 0 && slot.SKU == 0 && slot.Type != 6
		case "skc":
			path = fmt.Sprintf("skc_list.%d.image_info", slot.SKC)
			valid = slot.SKC >= 0 && slot.SKC < len(p.SKCs) && slot.SKU == 0
		case "sku":
			path = fmt.Sprintf("skc_list.%d.sku_list.%d.image_info", slot.SKC, slot.SKU)
			valid = slot.SKC >= 0 && slot.SKC < len(p.SKCs) && slot.SKU >= 0 && slot.SKU < len(p.SKCs[slot.SKC].SKUs) && slot.Type == 1 && slot.Sort == 1
		case "detail":
			path = fmt.Sprintf("skc_list.%d.site_detail_image_info_list", slot.SKC)
			valid = slot.SKC >= 0 && slot.SKC < len(p.SKCs) && slot.SKU == 0 && slot.Type == 7
		default:
			valid = false
		}
		approved, ok := assets[slot.AssetID]
		if !valid || !ok || slot.Sort < 1 || slot.Sort > 100 || slot.Type != 1 && slot.Type != 2 && slot.Type != 5 && slot.Type != 6 && slot.Type != 7 || slot.Type == 1 && slot.Sort != 1 {
			b.issue(path, "invalid", "图片位置、类型、顺序或批准身份无效")
			allRemote = false
			continue
		}
		key := fmt.Sprintf("%s/%d", path, slot.Sort)
		if sortSeen[key] {
			b.issue(path, "invalid", "同一图片组的顺序不得重复")
		}
		sortSeen[key] = true
		width, height := approved.Width, approved.Height
		imageURL, remoteMatched := approved.URL, false
		for _, observation := range observations {
			if observation.AssetID == slot.AssetID && observation.Type == slot.Type {
				if observation.SourceURL != approved.URL {
					b.issue(path, "image_evidence_conflict", "图片核实结果不属于本次批准的原图")
					continue
				}
				if observation.Width > 0 && observation.Height > 0 {
					width, height = observation.Width, observation.Height
				}
				responseHash, err := hex.DecodeString(observation.ResponseHash)
				if observation.RemoteURL != "" && IsOfficialImageReference(observation.RemoteURL) && err == nil && len(responseHash) == 32 && ValidImageObservation(observation) {
					imageURL = observation.RemoteURL
					remoteMatched = true
				}
			}
		}
		allRemote = allRemote && remoteMatched
		validSize := false
		switch slot.Type {
		case 1, 2:
			validSize = width == 1340 && height == 1785 || width == height && width >= 900 && width <= 2200
		case 5:
			validSize = width == height && width >= 900 && width <= 2200
			if slot.Group == "spu" {
				validSize = width == 1200 && height == 1200
			}
		case 6:
			validSize = width == 80 && height == 80
		case 7:
			validSize = width > 900 && height > 900 && width*4 == height*3
		}
		if !validSize {
			b.issue(path, "image_dimensions", "图片尺寸不符合所选图片类型的规范，或尚未核实真实尺寸")
		}
		groups[path] = append(groups[path], model.ProductImage{Sort: slot.Sort, Type: slot.Type, URL: imageURL})
	}
	for path, images := range groups {
		sort.Slice(images, func(i, j int) bool { return images[i].Sort < images[j].Sort })
		groups[path] = images
	}
	validateGroup := func(path string, mainRequired, detailRequired, detailAllowed, squareRequired, squareAllowed, pieceRequired, single bool) {
		counts := map[int]int{}
		for _, image := range groups[path] {
			counts[image.Type]++
		}
		if counts[1] > 1 || mainRequired && counts[1] != 1 || counts[2] > 10 || detailRequired && counts[2] < 1 || !detailAllowed && counts[2] > 0 || counts[5] > 1 || squareRequired && counts[5] != 1 || !squareAllowed && counts[5] > 0 || counts[6] > 1 || pieceRequired && counts[6] != 1 || single && counts[2] > 0 {
			b.issue(path, "missing_images", "按当前店铺规范补齐所需图片，并移除不允许的类型")
		}
	}
	if newScheme {
		spuPath := "image_info"
		spuImages := groups[spuPath]
		if len(spuImages) > 0 || rules["spu_image_detail_required"] || rules["spu_image_square_required"] {
			validateGroup(spuPath, rules["spu_image_detail_required"] || len(spuImages) > 0, rules["spu_image_detail_required"] && !rules["spu_image_detail_single"], rules["spu_image_detail_show"], rules["spu_image_square_required"], rules["spu_image_square_show"], false, rules["spu_image_detail_single"])
		}
		if len(spuImages) > 0 {
			p.ImageInfo = &model.ImageInfo{Images: spuImages}
		}
	}
	for i := range p.SKCs {
		skc := &p.SKCs[i]
		path := fmt.Sprintf("skc_list.%d.image_info", i)
		if newScheme {
			validateGroup(path, true, rules["skc_image_detail_required"] && !rules["skc_image_detail_single"], rules["skc_image_detail_show"], rules["skc_image_square_required"], rules["skc_image_square_show"], len(p.SKCs) > 1, rules["skc_image_detail_single"])
		} else {
			validateGroup(path, true, true, true, true, true, len(p.SKCs) > 1, false)
		}
		skc.ImageInfo = model.ImageInfo{Images: groups[path]}
		allSKURequired := rules["sku_image_required"]
		for j, sku := range skc.SKUs {
			allSKURequired = allSKURequired || len(groups[fmt.Sprintf("skc_list.%d.sku_list.%d.image_info", i, j)]) > 0 || sku.Quantity != nil && sku.Quantity.Quantity >= 2
		}
		for j := range skc.SKUs {
			skuPath := fmt.Sprintf("skc_list.%d.sku_list.%d.image_info", i, j)
			if allSKURequired {
				validateGroup(skuPath, true, false, false, false, false, false, true)
			}
			if images := groups[skuPath]; len(images) > 0 {
				skc.SKUs[j].ImageInfo = &model.ImageInfo{Images: images}
			}
		}
	}
	for i := range p.SKCs {
		detailPath := fmt.Sprintf("skc_list.%d.site_detail_image_info_list", i)
		if images := groups[detailPath]; len(images) > 0 {
			if len(images) > 10 {
				b.issue(detailPath, "invalid", "站点详情图最多十张")
			}
			details := []model.DetailImage{}
			for _, image := range images {
				details = append(details, model.DetailImage{Sort: image.Sort, URL: image.URL})
			}
			p.SKCs[i].SiteDetailImages = []model.SiteDetailImages{{Sites: []string{"shein-us"}, Images: details}}
		}
	}
	return allRemote
}
