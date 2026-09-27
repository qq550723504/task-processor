package browser

import "github.com/mxschmitt/playwright-go"

// Page-side extraction bound. These caps are applied INSIDE the page before the
// payload crosses the CDP boundary, so an oversized or hostile page cannot force
// the collector to materialize it (D8 / finding #13). The application layer's
// MaxAcquisitionCommandBytes is a separate, later guard on the command.
const (
	maxImages       = 64
	maxAttributes   = 128
	maxVariants     = 256
	maxVariantAttrs = 32
	maxStringLen    = 8192
	// maxPriceFacts bounds price facts page-side, before the CDP boundary.
	maxPriceFacts = 64
)

// challengeHostMarkers identify an anti-automation or authentication wall by
// host. Observed in real acceptance: after repeated anonymous requests 1688
// redirects detail pages to login.taobao.com / login.1688.com, which carries no
// product and no slider, so it must be reported as a challenge rather than as an
// unknown page shape.
var challengeHostMarkers = []string{
	"login.taobao.com",
	"login.1688.com",
	"passport.1688.com",
	"_____tmd_____",
	"punish",
}

// detectChallenge reports whether the loaded page is an anti-automation
// interstitial or an authentication wall rather than a product page. It is
// deliberately conservative: it only returns true on clear, well-known markers
// so a real product page is never misclassified.
func detectChallenge(page playwright.Page) (bool, error) {
	if page == nil {
		return false, nil
	}
	// Only challenge-specific title markers count. Generic words such as
	// "robot" or "verify" occur in ordinary product titles ("Robot Vacuum
	// Cleaner"), and treating those as a challenge would permanently fail a
	// legitimate listing.
	title, _ := page.Title()
	lower := toLower(title)
	markers := []string{"验证码", "请登录", "安全验证", "滑动验证", "captcha", "punish", "滑动验证验证", "异常流量"}
	for _, m := range markers {
		if contains(lower, m) {
			return true, nil
		}
	}
	// A redirect away from the product host, or 1688's anti-bot path marker, is a
	// challenge even when the page title looks like a normal site title.
	currentURL := toLower(page.URL())
	for _, m := range challengeHostMarkers {
		if contains(currentURL, m) {
			return true, nil
		}
	}
	return false, nil
}

func toLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func contains(hay, needle string) bool {
	if needle == "" {
		return false
	}
	n, h := len(needle), len(hay)
	if n > h {
		return false
	}
	for i := 0; i+n <= h; i++ {
		if hay[i:i+n] == needle {
			return true
		}
	}
	return false
}

// stealthScript is the minimal anti-detection init script, ported from the
// operator's fingerprint browser. It only suppresses the well-known automation
// tells; it does not spoof hardware.
func stealthScript() string {
	return `(() => {
  const override = (obj, key, value) => {
    try { Object.defineProperty(obj, key, { get: () => value, configurable: true }); } catch (_) {}
  };
  override(Navigator.prototype, 'webdriver', false);
  override(Navigator.prototype, 'languages', ['zh-CN', 'zh', 'en-US', 'en']);
  override(Navigator.prototype, 'language', 'zh-CN');
  override(Navigator.prototype, 'platform', 'Win32');
})();`
}

// extractScript returns the bounded, page-side projection of window.context
// (with a window.__INIT_DATA fallback for custom items). Field paths are
// ported from the operator's extractors so evidence semantics match the static
// HTTP provider.
//
// Every collection and every string is capped before returning, and every cap
// that actually clips records the affected field in truncatedFields. The Go
// decoder turns those into explicit warnings and missing facts, so a clipped
// value is never published as though it were the exact source fact.
//
// Numeric identifiers and prices are additionally guarded: JavaScript cannot
// represent every integer exactly, so a number outside the safe-integer range
// is reported as an imprecise field and dropped rather than silently rewritten
// (String(9007199254740993) would otherwise publish ...992).
func extractScript() string {
	return `() => {
  const CAP = { images: ` + itoa(maxImages) + `, attributes: ` + itoa(maxAttributes) + `, variants: ` + itoa(maxVariants) + `, variantAttrs: ` + itoa(maxVariantAttrs) + `, priceFacts: ` + itoa(maxPriceFacts) + `, str: ` + itoa(maxStringLen) + ` };
  const out = { offerId: '', title: '', description: '', images: [], attributes: [], variants: [], priceFacts: [], truncatedFields: [] };
  const seenTrunc = {};
  const markTrunc = (field) => { if (!seenTrunc[field]) { seenTrunc[field] = true; out.truncatedFields.push(field); } };
  // A clipped value is NOT the source fact. Returning a shortened string would
  // let the truncated value be published as if it were exact, so an oversized
  // value is dropped entirely and the field is reported instead.
  const clip = (s, field) => {
    if (typeof s !== 'string') return '';
    if (s.length <= CAP.str) return s;
    markTrunc(field);
    return '';
  };
  // Exact numeric text, or '' when the value cannot be represented exactly.
  // A fractional number such as 9.5 is exactly representable and must be kept;
  // only an integer beyond the safe-integer range has lost precision, and only
  // that case is rejected. JavaScript number-to-string is a shortest
  // round-trip, so a kept fraction faithfully reports the parsed value.
  const exactNum = (v, field) => {
    if (v === null || v === undefined || v === '') return '';
    if (typeof v === 'number') {
      if (!Number.isFinite(v)) { markTrunc(field); return ''; }
      if (Number.isInteger(v) && !Number.isSafeInteger(v)) { markTrunc(field); return ''; }
      return String(v);
    }
    if (typeof v === 'string') {
      // A huge numeric string would otherwise cross CDP unbounded.
      if (v.length > CAP.str) { markTrunc(field); return ''; }
      return v;
    }
    markTrunc(field);
    return '';
  };
  // A capped collection is NOT the source fact. Slicing it would publish a lossy
  // prefix as if it were complete, so an oversized collection is dropped whole
  // and the field is reported.
  const cap = (arr, n, field) => {
    if (arr.length > n) { markTrunc(field); return []; }
    return arr;
  };
  // Protocol-relative URLs are a known 1688 image shape. MapAcquisitionEvidence
  // requires an https scheme, so they must be normalized rather than emitted as
  // "//host/..." which would fail the whole acquisition.
  const absUrl = (u) => (typeof u === 'string' && u.indexOf('//') === 0) ? 'https:' + u : u;
  const pushAttrs = (list, field) => {
    for (const a of cap(list, CAP.attributes, field)) {
      if (a && a.name && a.value && a.name !== a.value) {
        out.attributes.push({ name: clip(a.name, field), value: clip(a.value, field) });
      }
    }
  };
  const readSku = (sku) => {
    if (!sku) return;
    const props = Array.isArray(sku.skuProps) ? sku.skuProps : [];
    const propNames = props.map((p) => (p && p.prop ? p.prop : ''));
    const map = sku.skuInfoMap || {};
    for (const key in map) {
      if (out.variants.length >= CAP.variants) { markTrunc('variants'); break; }
      const entry = map[key] || {};
      const parts = String(key).split('&gt;');
      const attrsFor = [];
      for (let i = 0; i < parts.length; i++) {
        if (i >= CAP.variantAttrs) { markTrunc('variant_attributes'); break; }
        const name = propNames[i];
        // Never invent a name the source did not supply: an unmatched segment is
        // reported as imprecise rather than persisted under a fabricated name.
        if (!name) { markTrunc('variant_attributes'); continue; }
        attrsFor.push({ name: clip(name, 'variant_attributes'), value: clip(parts[i], 'variant_attributes') });
      }
      // If any attribute of this variant was clipped, publishing the remaining
      // subset would misstate the variant. Drop the whole variant.
      if (attrsFor.some((a) => !a.name || !a.value)) { markTrunc('variants'); continue; }
      const v = { sourceId: exactNum(entry.skuId, 'variant_source_id'), attributes: attrsFor, price: null };
      const amount = exactNum(entry.price === undefined || entry.price === null ? entry.discountPrice : entry.price, 'variant_price');
      if (amount) {
        v.price = { amount: amount, currency: clip(entry.currency || '', 'variant_price'), minQuantity: '' };
      }
      out.variants.push(v);
    }
  };
  const readPrices = (data) => {
    for (const k in data) {
      const item = data[k];
      const ranges = item && item.fields && item.fields.finalPriceModel && item.fields.finalPriceModel.tradeWithoutPromotion && item.fields.finalPriceModel.tradeWithoutPromotion.offerPriceRanges;
      if (!Array.isArray(ranges)) continue;
      for (const r of ranges) {
        if (out.priceFacts.length >= CAP.priceFacts) { markTrunc('price_facts'); break; }
        if (!r || r.price === undefined || r.price === null) continue;
        const amount = exactNum(r.price, 'price_facts');
        if (!amount) continue;
        out.priceFacts.push({
          amount: amount,
          currency: clip(r.currency || '', 'price_facts'),
          minQuantity: exactNum(r.beginAmount, 'price_facts')
        });
      }
    }
  };

  // The description is read for both supported page shapes, so a custom-item
  // page that supplies a meta description is not recorded as missing one.
  {
    const meta = document.querySelector('meta[name="description"]');
    if (meta && meta.getAttribute('content')) out.description = clip(meta.getAttribute('content'), 'description');
  }

  const ctx = (typeof window.context !== 'undefined' && window.context && window.context.result) ? window.context.result : null;
  const init = (typeof window.__INIT_DATA !== 'undefined' && window.__INIT_DATA && window.__INIT_DATA.data) ? window.__INIT_DATA.data : null;

  if (ctx) {
    const data = ctx.data || {};
    if (data.productTitle && data.productTitle.fields) {
      out.title = clip(data.productTitle.fields.title || '', 'title');
    }
    if (data.Root && data.Root.fields && data.Root.fields.dataJson && data.Root.fields.dataJson.tempModel) {
      out.offerId = exactNum(data.Root.fields.dataJson.tempModel.offerId, 'offer_id');
    }
    // A valid page may omit tempModel while the canonical detail URL already
    // identifies the offer. Recovering it keeps the acquisition usable instead of
    // rejecting a page that has a title and product data.
    if (!out.offerId) {
      const m = /\/offer\/(\d+)\.html/.exec(String(location.pathname || ''));
      if (m) out.offerId = m[1];
    }
    if (data.gallery && data.gallery.fields && Array.isArray(data.gallery.fields.offerImgList)) {
      for (const u of cap(data.gallery.fields.offerImgList, CAP.images, 'images')) {
        if (typeof u === 'string' && u) out.images.push(clip(absUrl(u), 'images'));
      }
    }
    const attrs = ctx.global && ctx.global.globalData && ctx.global.globalData.model && ctx.global.globalData.model.offerDetail && ctx.global.globalData.model.offerDetail.featureAttributes;
    if (Array.isArray(attrs)) pushAttrs(attrs, 'attributes');
    if (data.Root && data.Root.fields && data.Root.fields.dataJson) {
      readSku(data.Root.fields.dataJson.skuModel);
    }
    readPrices(data);
    // Known standard-page price shapes when offerPriceRanges is absent, ported
    // from the operator's legacy price extractor.
    if (out.priceFacts.length === 0) {
      const dj = data.Root && data.Root.fields && data.Root.fields.dataJson;
      const op = dj && dj.orderParamModel && dj.orderParamModel.orderParam;
      const range = op && op.skuParam && op.skuParam.skuRangePrices;
      const collect = (list) => {
        for (const r of cap(list, CAP.priceFacts, 'price_facts')) {
          if (out.priceFacts.length >= CAP.priceFacts) break;
          if (!r) continue;
          const amount = exactNum(r.price, 'price_facts');
          if (!amount) continue;
          out.priceFacts.push({ amount: amount, currency: clip(r.currency || '', 'price_facts'), minQuantity: exactNum(r.beginAmount, 'price_facts') });
        }
      };
      if (Array.isArray(range)) collect(range);
      for (const k in data) {
        const f = data[k] && data[k].fields;
        if (f && f.priceModel && Array.isArray(f.priceModel.currentPrices)) { collect(f.priceModel.currentPrices); break; }
      }
    }
  } else if (init) {
    // The authoritative global SKU model is primary and is read before any data
    // block, matching the legacy foundSkuModel guard.
    const g = (typeof window.__INIT_DATA !== 'undefined' && window.__INIT_DATA && window.__INIT_DATA.globalData) ? window.__INIT_DATA.globalData : null;
    if (g) {
      if (g.skuModel) readSku(g.skuModel);
      else if (g.nySkuModel) readSku(g.nySkuModel);
      else if (g.skuModelOrigin) readSku(g.skuModelOrigin);
    }
    for (const k in init) {
      const block = init[k];
      if (!block || typeof block !== 'object') continue;
      const d = block.data || {};
      if (d.offerInfoModel && !out.offerId) {
        out.offerId = exactNum(d.offerInfoModel.offerId, 'offer_id');
        if (!out.title) out.title = clip(d.offerInfoModel.title || '', 'title');
      }
      // Known direct identity shape on custom-item blocks.
      if (!out.offerId && d.offerId !== undefined && d.offerId !== null) {
        out.offerId = exactNum(d.offerId, 'offer_id');
      }
      if (!out.title && d.title) out.title = clip(d.title, 'title');
      if (Array.isArray(d.offerImgList) && out.images.length === 0) {
        for (const u of cap(d.offerImgList, CAP.images, 'images')) if (typeof u === 'string' && u) out.images.push(clip(absUrl(u), 'images'));
      }
      if (Array.isArray(d.propsList)) pushAttrs(d.propsList, 'attributes');
      else if (Array.isArray(d)) {
        // Known shape where the attribute list is the block data array itself.
        pushAttrs(d.filter((a) => a && a.name && a.value), 'attributes');
      }
      // Block-local models are a fallback only; the authoritative global model is
      // read before this loop, so a partial or differing block model cannot
      // pre-empt it, and reading both cannot duplicate variants (which
      // MapAcquisitionEvidence rejects as repeated source IDs).
      if (out.variants.length === 0) {
        if (d.skuModel) readSku(d.skuModel);
        else if (d.nySkuModel) readSku(d.nySkuModel);
        else if (d.skuModelOrigin) readSku(d.skuModelOrigin);
        else if (d.skuInfoMap) readSku({ skuInfoMap: d.skuInfoMap, skuProps: d.skuProps });
      }
    }
    if (g && g.offerInfoModel && !out.offerId) {
      out.offerId = exactNum(g.offerInfoModel.offerId, 'offer_id');
      if (!out.title) out.title = clip(g.offerInfoModel.title || '', 'title');
    }
    // Custom-item price shapes, ported from the operator's legacy price
    // extractor: the order-parameter SKU range prices, and each data block's
    // priceModel.currentPrices. readPrices() only understands the standard
    // item.fields.finalPriceModel nesting, so these needed explicit handling.
    if (g) {
      const op = g.orderParamModel && g.orderParamModel.orderParam;
      const range = op && op.skuParam && op.skuParam.skuRangePrices;
      if (Array.isArray(range)) {
        for (const r of cap(range, CAP.priceFacts, 'price_facts')) {
          if (out.priceFacts.length >= CAP.priceFacts) break;
          if (!r) continue;
          const amount = exactNum(r.price, 'price_facts');
          if (!amount) continue;
          out.priceFacts.push({ amount: amount, currency: clip(r.currency || '', 'price_facts'), minQuantity: exactNum(r.beginAmount, 'price_facts') });
        }
      }
    }
    // currentPrices is only a fallback: when the global range prices were already
    // read, appending this second representation would persist overlapping or
    // conflicting tiers as unrelated price facts, because AcquisitionPrice
    // carries no model or promotion discriminator.
    for (const k in (out.priceFacts.length === 0 ? init : {})) {
      const block = init[k];
      const d = block && block.data;
      if (!d || !d.priceModel || !Array.isArray(d.priceModel.currentPrices)) continue;
      for (const r of cap(d.priceModel.currentPrices, CAP.priceFacts, 'price_facts')) {
        if (out.priceFacts.length >= CAP.priceFacts) break;
        if (!r) continue;
        const amount = exactNum(r.price, 'price_facts');
        if (!amount) continue;
        out.priceFacts.push({ amount: amount, currency: clip(r.currency || '', 'price_facts'), minQuantity: exactNum(r.beginAmount, 'price_facts') });
      }
      break;
    }
    if (out.priceFacts.length === 0) readPrices(init);
  }
  if (out.images.length > CAP.images) out.images = cap(out.images, CAP.images, 'images');
  if (out.attributes.length > CAP.attributes) out.attributes = cap(out.attributes, CAP.attributes, 'attributes');
  if (out.variants.length > CAP.variants) out.variants = cap(out.variants, CAP.variants, 'variants');
  if (out.priceFacts.length > CAP.priceFacts) out.priceFacts = cap(out.priceFacts, CAP.priceFacts, 'price_facts');
  return out;
}`
}

// isAuthenticationWall reports whether the challenge is a login redirect rather
// than a slider challenge. Observed in real acceptance: 1688 answers repeated
// anonymous requests with a redirect to login.taobao.com. No automatic handling
// applies to that, so the caller reports it honestly instead of attempting a
// slider that cannot help.
func isAuthenticationWall(page playwright.Page) bool {
	if page == nil {
		return false
	}
	currentURL := toLower(page.URL())
	for _, m := range []string{"login.taobao.com", "login.1688.com", "passport.1688.com"} {
		if contains(currentURL, m) {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
