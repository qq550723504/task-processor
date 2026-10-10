package shein

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	o "task-processor/internal/marketplace/shein/observations"
	"task-processor/internal/storecenter"
	"time"
)

var observationDecimal = regexp.MustCompile(`^[0-9]{1,20}(\.[0-9]{1,8})?$`)
var observationDigits = regexp.MustCompile(`^[1-9][0-9]{0,29}$`)

func scalar(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if raw[0] == '"' {
		if json.Unmarshal(raw, &text) != nil {
			return "", o.ErrUnavailable
		}
	} else {
		text = string(raw)
	}
	if !o.Text(text, 128) {
		return "", o.ErrUnavailable
	}
	return text, nil
}
func priceValue(raw json.RawMessage) (string, error) {
	s, e := scalar(raw)
	if e != nil || s != "" && !observationDecimal.MatchString(s) {
		return "", o.ErrUnavailable
	}
	return s, nil
}
func observedPrice(raw, special json.RawMessage, currency string) (o.Price, error) {
	v, e := priceValue(raw)
	if e != nil {
		return o.Price{}, e
	}
	sale, e := priceValue(special)
	if e != nil {
		return o.Price{}, e
	}
	if sale != "" {
		f, e := strconv.ParseFloat(sale, 64)
		if e != nil {
			return o.Price{}, o.ErrUnavailable
		}
		if f == 0 {
			sale = ""
		}
	}
	if currency != "" && (len(currency) != 3 || strings.ContainsFunc(currency, func(c rune) bool { return c < 'A' || c > 'Z' })) {
		return o.Price{}, o.ErrUnavailable
	}
	return o.Price{Currency: currency, Value: v, Special: sale}, nil
}
func observedTime(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	if len(s) > 40 {
		return "", o.ErrUnavailable
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999Z0700"} {
		t, e := time.Parse(layout, s)
		if e == nil {
			return t.UTC().Format(time.RFC3339Nano), nil
		}
	}
	return "", o.ErrUnavailable
}
func observedImage(s string) string {
	u, e := url.Parse(s)
	if e != nil || len(s) > 2048 || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return ""
	}
	return s
}

type observedEnvelope[T any] struct {
	Code    json.RawMessage `json:"code"`
	Info    *T              `json:"info"`
	Message string          `json:"msg"`
}

func observationInfo[T any](raw []byte, orderList bool) (T, error) {
	return observationResult[T](raw, orderList, false)
}
func observationResult[T any](raw []byte, orderList, allowMissingInfo bool) (T, error) {
	var zero T
	var r observedEnvelope[T]
	if decodeGoods(raw, &r) != nil {
		return zero, o.ErrUnavailable
	}
	code, e := scalar(r.Code)
	if e != nil || code != "0" || r.Info == nil && !allowMissingInfo {
		if orderList && code == "9999400" && strings.Contains(r.Message, "10000") {
			return zero, o.ErrWindowLimit
		}
		return zero, o.ErrUnavailable
	}
	if r.Info == nil {
		return zero, nil
	}
	return *r.Info, nil
}

type observedTitle struct {
	Language string `json:"language"`
	Title    string `json:"title"`
}
type observedSite struct {
	Site   string `json:"subSite"`
	Status *int   `json:"status"`
}
type observedRawPrice struct {
	Site     string          `json:"site"`
	Currency string          `json:"currency"`
	Base     json.RawMessage `json:"basePrice"`
	Special  json.RawMessage `json:"specialPrice"`
	Cost     json.RawMessage `json:"cost"`
}
type observedRawSKU struct {
	ID        string             `json:"skuCode"`
	Seller    string             `json:"supplierSku"`
	Prices    []observedRawPrice `json:"priceList"`
	Costs     []observedRawPrice `json:"costList"`
	Inventory []struct {
		ID       string `json:"warehouseId"`
		Quantity *int64 `json:"inventoryNum"`
	} `json:"inventoryList"`
}
type observedRawSKC struct {
	ID     string           `json:"skcName"`
	Seller string           `json:"supplierCode"`
	Image  string           `json:"skcMainPicUrl"`
	Titles []observedTitle  `json:"skcTitle"`
	Sites  []observedSite   `json:"skcSiteShelfStatusList"`
	SKUs   []observedRawSKU `json:"skuList"`
}
type observedRawProduct struct {
	ID   string           `json:"spuName"`
	SKCs []observedRawSKC `json:"skcList"`
}

func (c *OfficialClient) QueryObservedProducts(ctx context.Context, k storecenter.OfficialMerchantCredential, page int) (o.ProductPage, error) {
	if page < 1 || page > 5000 {
		return o.ProductPage{}, o.ErrInvalid
	}
	body, _ := json.Marshal(struct {
		Page      int      `json:"pageNum"`
		Size      int      `json:"pageSize"`
		Languages []string `json:"languageList"`
	}{page, 10, []string{"en"}})
	raw, e := c.goodsPost(ctx, "/open-api/goods/searchProduct", k, body)
	if e != nil {
		return o.ProductPage{}, o.ErrUnavailable
	}
	info, e := observationInfo[struct {
		Meta *struct {
			Count *int `json:"count"`
		} `json:"meta"`
		Data []observedRawProduct `json:"data"`
	}](raw, false)
	if e != nil || info.Meta == nil || info.Meta.Count == nil || *info.Meta.Count < 0 || info.Data == nil || len(info.Data) > 10 {
		return o.ProductPage{}, o.ErrUnavailable
	}
	out := o.ProductPage{Total: *info.Meta.Count, Items: []o.Product{}}
	seen := map[string]bool{}
	for _, p := range info.Data {
		if !o.Identity(p.ID) || seen[p.ID] || p.SKCs == nil || len(p.SKCs) > 100 {
			return o.ProductPage{}, o.ErrUnavailable
		}
		seen[p.ID] = true
		target := o.Product{ID: p.ID, SKCs: []o.SKC{}}
		skcs := map[string]bool{}
		for _, s := range p.SKCs {
			if !o.Identity(s.ID) || skcs[s.ID] || !o.Text(s.Seller, 800) || len(s.Sites) > 100 || len(s.Titles) > 20 || s.SKUs == nil || len(s.SKUs) > 400 {
				return o.ProductPage{}, o.ErrUnavailable
			}
			skcs[s.ID] = true
			result := o.SKC{ID: s.ID, SellerCode: s.Seller, ImageURL: observedImage(s.Image), SKUs: []o.SKU{}}
			found := false
			sites := map[string]bool{}
			for _, site := range s.Sites {
				if !o.Identity(site.Site) || sites[site.Site] || site.Status != nil && (*site.Status < 0 || *site.Status > 1000000) {
					return o.ProductPage{}, o.ErrUnavailable
				}
				sites[site.Site] = true
				if site.Site == "shein-us" {
					found = true
					result.Site = "shein-us"
					result.SiteStatus = site.Status
				}
			}
			if !found && len(s.Sites) > 0 {
				continue
			}
			for _, t := range s.Titles {
				if !o.Text(t.Title, 1000) || !o.Text(t.Language, 32) {
					return o.ProductPage{}, o.ErrUnavailable
				}
				if result.Title == "" || t.Language == "en" {
					result.Title = t.Title
					if t.Language == "en" {
						break
					}
				}
			}
			skus := map[string]bool{}
			for _, sku := range s.SKUs {
				if !o.Identity(sku.ID) || skus[sku.ID] || !o.Text(sku.Seller, 800) || len(sku.Prices) > 100 || len(sku.Costs) > 100 || len(sku.Inventory) > 200 {
					return o.ProductPage{}, o.ErrUnavailable
				}
				skus[sku.ID] = true
				value := o.SKU{ID: sku.ID, SellerSKU: sku.Seller, Prices: []o.Price{}, Costs: []o.Price{}, Inventory: []o.Inventory{}}
				for _, p := range sku.Prices {
					if p.Site != "shein-us" {
						continue
					}
					price, e := observedPrice(p.Base, p.Special, p.Currency)
					if e != nil {
						return o.ProductPage{}, e
					}
					value.Prices = append(value.Prices, price)
				}
				for _, p := range sku.Costs {
					price, e := observedPrice(p.Cost, nil, p.Currency)
					if e != nil {
						return o.ProductPage{}, e
					}
					value.Costs = append(value.Costs, price)
				}
				warehouses := map[string]bool{}
				for _, i := range sku.Inventory {
					if !o.Identity(i.ID) || warehouses[i.ID] || i.Quantity != nil && (*i.Quantity < 0 || *i.Quantity > 9007199254740991) {
						return o.ProductPage{}, o.ErrUnavailable
					}
					warehouses[i.ID] = true
					value.Inventory = append(value.Inventory, o.Inventory{WarehouseID: i.ID, Quantity: i.Quantity})
				}
				result.SKUs = append(result.SKUs, value)
			}
			target.SKCs = append(target.SKCs, result)
		}
		out.Items = append(out.Items, target)
	}
	return out, nil
}
func (c *OfficialClient) QueryObservedOrders(ctx context.Context, k storecenter.OfficialMerchantCredential, w o.Window, page int) (o.OrderPage, error) {
	if page < 1 || page > 333 || w.Start.IsZero() || w.End.IsZero() || w.Start.After(w.End) || w.End.Sub(w.Start) > 48*time.Hour {
		return o.OrderPage{}, o.ErrInvalid
	}
	if c == nil {
		return o.OrderPage{}, o.ErrUnavailable
	}
	if strings.HasSuffix(c.application.Version, ":fully_managed") {
		return o.OrderPage{}, o.ErrUnsupported
	}
	zone := time.FixedZone("UTC+8", 8*3600)
	body, _ := json.Marshal(struct {
		Type      int    `json:"queryType"`
		Warehouse int    `json:"queryOrderType"`
		Start     string `json:"startTime"`
		End       string `json:"endTime"`
		Page      int    `json:"page"`
		Size      int    `json:"pageSize"`
	}{1, 4, w.Start.In(zone).Format("2006-01-02 15:04:05"), w.End.In(zone).Format("2006-01-02 15:04:05"), page, 30})
	raw, e := c.goodsPost(ctx, "/open-api/order/order-list", k, body)
	if e != nil {
		return o.OrderPage{}, o.ErrUnavailable
	}
	info, e := observationInfo[struct {
		Count  *int `json:"count"`
		Orders []struct {
			ID      string          `json:"orderNo"`
			Status  json.RawMessage `json:"orderStatus"`
			Created string          `json:"orderCreateTime"`
			Updated string          `json:"orderUpdateTime"`
		} `json:"orderList"`
	}](raw, true)
	if e != nil {
		return o.OrderPage{}, e
	}
	if info.Orders == nil || len(info.Orders) > 30 || info.Count != nil && *info.Count < 0 {
		return o.OrderPage{}, o.ErrUnavailable
	}
	out := o.OrderPage{ReportedCount: info.Count, Items: []o.OrderRef{}}
	seen := map[string]bool{}
	for _, v := range info.Orders {
		status, e := observedInt(v.Status)
		created, ce := time.ParseInLocation("2006-01-02 15:04:05", v.Created, zone)
		updated, ue := time.ParseInLocation("2006-01-02 15:04:05", v.Updated, zone)
		if !o.Identity(v.ID) || seen[v.ID] || e != nil || status == nil || ce != nil || ue != nil || created.Before(w.Start) || created.After(w.End) {
			return o.OrderPage{}, o.ErrUnavailable
		}
		seen[v.ID] = true
		out.Items = append(out.Items, o.OrderRef{ID: v.ID, Status: *status, CreatedAt: created.UTC(), UpdatedAt: updated.UTC()})
	}
	return out, nil
}
func observedInt(raw json.RawMessage) (*int, error) {
	s, e := scalar(raw)
	if e != nil {
		return nil, e
	}
	if s == "" {
		return nil, nil
	}
	n, e := strconv.Atoi(s)
	if e != nil || n < 0 || n > 1000000 {
		return nil, o.ErrUnavailable
	}
	return &n, nil
}

type observedRawItem struct {
	ID       json.RawMessage `json:"goodsId"`
	SKU      string          `json:"skuCode"`
	Seller   string          `json:"sellerSku"`
	Title    string          `json:"goodsTitle"`
	Image    string          `json:"spuPicURL"`
	Status   json.RawMessage `json:"newGoodsStatus"`
	Exchange json.RawMessage `json:"goodsExchangeTag"`
}
type observedRawDetail struct {
	ID       string            `json:"orderNo"`
	Site     string            `json:"salesSite"`
	Status   json.RawMessage   `json:"orderStatus"`
	Stock    json.RawMessage   `json:"stockMode"`
	Type     json.RawMessage   `json:"orderType"`
	Tag      json.RawMessage   `json:"orderTag"`
	Reasons  []int             `json:"unProcessReason"`
	Items    []observedRawItem `json:"orderGoodsInfoList"`
	Packages []struct {
		ID      string          `json:"packageNo"`
		Waybill string          `json:"waybillNo"`
		Carrier string          `json:"carrier"`
		Label   json.RawMessage `json:"packageLabel"`
	} `json:"packageWaybillList"`
	Currency     string          `json:"orderCurrency"`
	Amount       json.RawMessage `json:"productTotalPrice"`
	SaleCurrency string          `json:"saleCurrency"`
	Cost         json.RawMessage `json:"totalCostPrice"`
	Created      string          `json:"orderTime"`
	Updated      string          `json:"orderMsgUpdateTime"`
	Delivery     string          `json:"needDeliveryTime"`
	Handover     string          `json:"requestHandoverTime"`
	Collect      string          `json:"expectedCollectTime"`
}

func (c *OfficialClient) QueryObservedOrderDetails(ctx context.Context, k storecenter.OfficialMerchantCredential, ids []string) ([]o.Order, error) {
	if len(ids) < 1 || len(ids) > 30 {
		return nil, o.ErrInvalid
	}
	requested := map[string]bool{}
	for _, id := range ids {
		if !o.Identity(id) || requested[id] {
			return nil, o.ErrInvalid
		}
		requested[id] = true
	}
	if c == nil {
		return nil, o.ErrUnavailable
	}
	if strings.HasSuffix(c.application.Version, ":fully_managed") {
		return nil, o.ErrUnsupported
	}
	body, _ := json.Marshal(struct {
		IDs []string `json:"orderNoList"`
	}{ids})
	raw, e := c.goodsPost(ctx, "/open-api/order/order-detail", k, body)
	if e != nil {
		return nil, o.ErrUnavailable
	}
	data, e := observationInfo[[]observedRawDetail](raw, false)
	if e != nil || len(data) != len(ids) {
		return nil, o.ErrUnavailable
	}
	out := []o.Order{}
	for _, v := range data {
		if !requested[v.ID] || !o.Text(v.Site, 128) || len(v.Items) > 1000 || len(v.Packages) > 1000 || len(v.Reasons) > 50 {
			return nil, o.ErrUnavailable
		}
		delete(requested, v.ID)
		result := o.Order{ID: v.ID, Site: v.Site, Reasons: append([]int{}, v.Reasons...), Items: []o.OrderItem{}, Packages: []o.Package{}}
		for _, pair := range []struct {
			raw    json.RawMessage
			target **int
		}{{v.Status, &result.Status}, {v.Stock, &result.StockMode}, {v.Type, &result.Type}, {v.Tag, &result.Tag}} {
			*pair.target, e = observedInt(pair.raw)
			if e != nil {
				return nil, e
			}
		}
		for _, reason := range result.Reasons {
			if reason < 0 || reason > 1000000 {
				return nil, o.ErrUnavailable
			}
		}
		amount, e := observedPrice(v.Amount, nil, v.Currency)
		if e != nil {
			return nil, e
		}
		if amount.Value != "" {
			result.Amount = &amount
		}
		cost, e := observedPrice(v.Cost, nil, v.SaleCurrency)
		if e != nil {
			return nil, e
		}
		if cost.Value != "" {
			result.SupplyCost = &cost
		}
		for _, pair := range []struct {
			raw    string
			target *string
		}{{v.Created, &result.CreatedAt}, {v.Updated, &result.UpdatedAt}, {v.Delivery, &result.NeedDeliveryAt}, {v.Handover, &result.HandoverAt}, {v.Collect, &result.ExpectedCollectAt}} {
			*pair.target, e = observedTime(pair.raw)
			if e != nil {
				return nil, e
			}
		}
		seen := map[string]bool{}
		for _, it := range v.Items {
			id, e := scalar(it.ID)
			if e != nil || !observationDigits.MatchString(id) || seen[id] || !o.Text(it.SKU, 128) || !o.Text(it.Seller, 800) || !o.Text(it.Title, 1000) {
				return nil, o.ErrUnavailable
			}
			seen[id] = true
			status, e := observedInt(it.Status)
			if e != nil {
				return nil, e
			}
			exchange, e := observedInt(it.Exchange)
			if e != nil {
				return nil, e
			}
			result.Items = append(result.Items, o.OrderItem{ID: id, SKU: it.SKU, SellerSKU: it.Seller, Title: it.Title, ImageURL: observedImage(it.Image), Status: status, ExchangeTag: exchange})
		}
		seen = map[string]bool{}
		for _, p := range v.Packages {
			label, e := scalar(p.Label)
			if e != nil || !o.Text(p.ID, 128) || !o.Text(p.Waybill, 128) || !o.Text(p.Carrier, 200) || p.ID != "" && seen[p.ID] {
				return nil, o.ErrUnavailable
			}
			if p.ID != "" {
				seen[p.ID] = true
			}
			result.Packages = append(result.Packages, o.Package{ID: p.ID, Waybill: p.Waybill, Carrier: p.Carrier, Label: label})
		}
		out = append(out, result)
	}
	return out, nil
}
func (c *OfficialClient) QueryObservedTrack(ctx context.Context, k storecenter.OfficialMerchantCredential, order, pkg string) ([]o.Track, error) {
	if !o.Identity(order) || !o.Identity(pkg) {
		return nil, o.ErrInvalid
	}
	if c == nil {
		return nil, o.ErrUnavailable
	}
	if strings.HasSuffix(c.application.Version, ":fully_managed") {
		return nil, o.ErrUnsupported
	}
	raw, e := c.goodsRequest(ctx, http.MethodGet, "/open-api/gsp/logistics-track", url.Values{"orderNo": {order}, "packageNo": {pkg}}.Encode(), k, nil)
	if e != nil {
		return nil, o.ErrUnavailable
	}
	info, e := observationResult[struct {
		Tracks []struct {
			Carrier string `json:"carrier"`
			Waybill string `json:"waybillNo"`
			Nodes   []struct {
				Description string `json:"description"`
				Code        string `json:"nodeCode"`
				Name        string `json:"nodeCodeName"`
				At          int64  `json:"updateTimeMillis"`
			} `json:"tracking"`
		} `json:"trackInfo"`
	}](raw, false, true)
	if e != nil || len(info.Tracks) > 50 {
		return nil, o.ErrUnavailable
	}
	out := []o.Track{}
	for _, t := range info.Tracks {
		if !o.Text(t.Carrier, 200) || !o.Text(t.Waybill, 128) || len(t.Nodes) > 1000 {
			return nil, o.ErrUnavailable
		}
		track := o.Track{Carrier: t.Carrier, Waybill: t.Waybill, Nodes: []o.TrackNode{}}
		for _, n := range t.Nodes {
			if !o.Text(n.Description, 2000) || !o.Text(n.Code, 128) || !o.Text(n.Name, 500) || n.At < 0 {
				return nil, o.ErrUnavailable
			}
			track.Nodes = append(track.Nodes, o.TrackNode{Description: n.Description, Code: n.Code, Name: n.Name, AtMillis: n.At})
		}
		out = append(out, track)
	}
	return out, nil
}
