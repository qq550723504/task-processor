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
)

// detectChallenge reports whether the loaded page is an anti-automation
// interstitial rather than a product page. It is deliberately conservative: it
// only returns true on clear, well-known markers so a real product page is
// never misclassified.
func detectChallenge(page playwright.Page) (bool, error) {
	if page == nil {
		return false, nil
	}
	title, _ := page.Title()
	lower := toLower(title)
	markers := []string{"验证码", "请登录", "安全验证", "captcha", "punish", "verify", "robot", "滑动验证"}
	for _, m := range markers {
		if contains(lower, m) {
			return true, nil
		}
	}
	// A punish URL pattern is a definitive challenge signal.
	if contains(toLower(page.URL()), "/_____tmd_____/punish") || contains(toLower(page.URL()), "punish?") {
		return true, nil
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
// HTTP provider. Every collection is capped before returning.
func extractScript() string {
	return `() => {
  const CAP = { images: ` + itoa(maxImages) + `, attributes: ` + itoa(maxAttributes) + `, variants: ` + itoa(maxVariants) + `, variantAttrs: ` + itoa(maxVariantAttrs) + `, str: ` + itoa(maxStringLen) + ` };
  const clip = (s) => (typeof s === 'string' ? s.slice(0, CAP.str) : '');
  const out = { offerId: '', title: '', description: '', images: [], attributes: [], variants: [], priceFacts: [], truncated: false };

  const ctx = (typeof window.context !== 'undefined' && window.context && window.context.result) ? window.context.result : null;
  const init = (typeof window.__INIT_DATA !== 'undefined' && window.__INIT_DATA && window.__INIT_DATA.data) ? window.__INIT_DATA.data : null;

  if (ctx) {
    const data = ctx.data || {};
    if (data.productTitle && data.productTitle.fields) {
      out.title = clip(data.productTitle.fields.title || '');
    }
    if (data.Root && data.Root.fields && data.Root.fields.dataJson && data.Root.fields.dataJson.tempModel) {
      out.offerId = String(data.Root.fields.dataJson.tempModel.offerId || '');
    }
    if (data.gallery && data.gallery.fields && Array.isArray(data.gallery.fields.offerImgList)) {
      for (const u of data.gallery.fields.offerImgList.slice(0, CAP.images)) {
        if (typeof u === 'string' && u) out.images.push(clip(u));
      }
      if (data.gallery.fields.offerImgList.length > CAP.images) out.truncated = true;
    }
    const attrs = ctx.global && ctx.global.globalData && ctx.global.globalData.model && ctx.global.globalData.model.offerDetail && ctx.global.globalData.model.offerDetail.featureAttributes;
    if (Array.isArray(attrs)) {
      for (const a of attrs.slice(0, CAP.attributes)) {
        if (a && a.name && a.value && a.name !== a.value) out.attributes.push({ name: clip(a.name), value: clip(a.value) });
      }
      if (attrs.length > CAP.attributes) out.truncated = true;
    }
    if (data.Root && data.Root.fields && data.Root.fields.dataJson && data.Root.fields.dataJson.skuModel) {
      const sku = data.Root.fields.dataJson.skuModel;
      const props = Array.isArray(sku.skuProps) ? sku.skuProps : [];
      const propNames = props.map((p) => p && p.prop ? p.prop : 'attr');
      const map = sku.skuInfoMap || {};
      let n = 0;
      for (const key in map) {
        if (n++ >= CAP.variants) { out.truncated = true; break; }
        const entry = map[key] || {};
        const parts = String(key).split('&gt;');
        const attrsFor = [];
        for (let i = 0; i < parts.length && i < CAP.variantAttrs; i++) {
          attrsFor.push({ name: clip(propNames[i] || ('attr' + i)), value: clip(parts[i]) });
        }
        const v = { sourceId: entry && entry.skuId != null ? String(entry.skuId) : '', attributes: attrsFor, price: null };
        if (entry && entry.price != null) {
          v.price = { amount: clip(String(entry.price)), currency: clip(entry.currency || ''), minQuantity: '' };
        }
        out.variants.push(v);
      }
    }
    for (const k in data) {
      const item = data[k];
      const ranges = item && item.fields && item.fields.finalPriceModel && item.fields.finalPriceModel.tradeWithoutPromotion && item.fields.finalPriceModel.tradeWithoutPromotion.offerPriceRanges;
      if (Array.isArray(ranges)) {
        for (const r of ranges) {
          if (r && r.price != null) {
            out.priceFacts.push({ amount: clip(String(r.price)), currency: '', minQuantity: clip(String(r.beginAmount == null ? '' : r.beginAmount)) });
          }
        }
      }
    }
  } else if (init) {
    // Custom-item fallback: window.__INIT_DATA carries the same facts under
    // component data blocks.
    for (const k in init) {
      const block = init[k];
      if (!block || typeof block !== 'object') continue;
      const d = block.data || {};
      if (d.offerInfoModel && !out.offerId) {
        out.offerId = String(d.offerInfoModel.offerId || '');
        if (!out.title) out.title = clip(d.offerInfoModel.title || '');
        if (!out.title) {
          const t = block.data && d.offerInfoModel ? d.offerInfoModel.title : '';
          out.title = clip(t);
        }
      }
      if (Array.isArray(d.offerImgList) && out.images.length === 0) {
        for (const u of d.offerImgList.slice(0, CAP.images)) if (typeof u === 'string' && u) out.images.push(clip(u));
      }
      if (Array.isArray(d.propsList)) {
        for (const p of d.propsList.slice(0, CAP.attributes)) {
          if (p && p.name && p.value) out.attributes.push({ name: clip(p.name), value: clip(p.value) });
        }
      }
    }
  }
  if (out.images.length > CAP.images) out.images = out.images.slice(0, CAP.images);
  if (out.attributes.length > CAP.attributes) out.attributes = out.attributes.slice(0, CAP.attributes);
  if (out.variants.length > CAP.variants) out.variants = out.variants.slice(0, CAP.variants);
  return out;
}`
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
