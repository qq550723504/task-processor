package listingkit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) rememberSheinSubmittedSizeAttributes(task *Task, action string) {
	cacheStore := resolveSheinResolutionCacheStore(s)
	if s == nil || cacheStore == nil || task == nil || task.Result == nil || task.Result.Shein == nil || strings.TrimSpace(action) != "publish" {
		return
	}
	task.Result.Shein = sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	req := buildSheinPublishRequest(task.Request)
	review := sheinpub.NormalizePublishedSizeAttributeReview(task.Result.Shein)
	if req == nil || task.Result.Shein == nil || review == nil || !review.Ready {
		return
	}
	key := sheinpub.SizeAttributeCacheKey(req, task.Result.Shein)
	if key == "" {
		return
	}
	now := time.Now()
	attachSizeAttributeCacheInfo(review, "manual_cache", key, true, sheinpub.ResolutionCacheHitSourcePublishRemembered, "stored", 0, &now)
	task.Result.Shein.SizeAttributes = review
	_ = cacheStore.SaveResolutionCache(context.Background(), &sheinpub.SheinResolutionCacheEntry{
		StoreID:        sheinpub.SizeAttributeStoreID(req),
		CacheKind:      sheinpub.ResolutionCacheKindSizeAttribute,
		CacheKey:       key,
		ShortKey:       sheinpub.SizeAttributeShortKey(key),
		Source:         "manual_cache",
		Manual:         true,
		SourceIdentity: sheinpub.SizeAttributeSourceIdentity(task.Result.Shein),
		ResolutionJSON: mustMarshalSheinSizeAttributeReview(review),
		UpdatedAt:      now,
		CreatedAt:      now,
	})
	logSizeAttributeCacheEvent("store", req, task.Result.Shein, review.Cache, logrus.Fields{
		"cache_kind": sheinpub.ResolutionCacheKindSizeAttribute,
		"size_facts": strings.Join(sheinpub.SortedSizeAttributeFacts(task.Result.Shein), ","),
	})
}

func (s *service) applyDefaultSheinSizeAttributes(req *GenerateRequest, pkg *sheinpub.Package) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if reason := sheinSizeAttributeCacheSkipReason(pkg); reason != "" {
		logSizeAttributeCacheEvent("skip", buildSheinPublishRequest(req), pkg, sheinSizeAttributeCacheInfo(pkg), logrus.Fields{"reason": reason})
		return
	}
	if cached := s.loadSheinSizeAttributeCache(req, pkg); cached != nil {
		sheinpub.ApplySizeAttributeReview(pkg, cached)
		return
	}
	review := sheinpub.NormalizePublishedSizeAttributeReview(pkg)
	if review != nil {
		pkg.SizeAttributes = review
	}
	logSizeAttributeCacheEvent("miss", buildSheinPublishRequest(req), pkg, sheinSizeAttributeCacheInfo(pkg), nil)
}

func (s *service) loadSheinSizeAttributeCache(req *GenerateRequest, pkg *sheinpub.Package) *sheinpub.SizeAttributeReview {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	buildReq := buildSheinPublishRequest(req)
	if s == nil || pkg == nil {
		return nil
	}
	cacheStore := resolveSheinResolutionCacheStore(s)
	if cacheStore == nil {
		return nil
	}
	key := sheinpub.SizeAttributeCacheKey(buildReq, pkg)
	if key == "" {
		return nil
	}
	entry, err := cacheStore.GetResolutionCache(context.Background(), sheinpub.ResolutionCacheKindSizeAttribute, sheinpub.SizeAttributeStoreID(buildReq), key)
	if err != nil || entry == nil {
		return nil
	}
	review := sheinpub.DecodeSizeAttributeCacheEntry(entry)
	review = sheinpub.ReconcileSizeAttributeCacheReview(pkg, review)
	if !sheinpub.SizeAttributeReviewApplicable(pkg, review) {
		return nil
	}
	attachSizeAttributeCacheInfo(review, cacheEntrySourceLabel(entry), entry.CacheKey, entry.Manual, pricingCacheHitSource(entry), "hit", entry.HitCount, &entry.UpdatedAt)
	logSizeAttributeCacheEvent("hit", buildReq, pkg, review.Cache, logrus.Fields{
		"cache_kind": sheinpub.ResolutionCacheKindSizeAttribute,
		"hit_count":  entry.HitCount,
	})
	return review
}

func (s *service) clearSheinSizeAttributeCache(req *sheinpub.BuildRequest, pkg *sheinpub.Package) error {
	cacheStore := resolveSheinResolutionCacheStore(s)
	if s == nil || cacheStore == nil {
		return nil
	}
	key := sheinpub.SizeAttributeCacheKey(req, pkg)
	if key == "" {
		return nil
	}
	if pkg != nil && pkg.SizeAttributes != nil {
		pkg.SizeAttributes.Cache = nil
	}
	return cacheStore.DeleteResolutionCache(context.Background(), sheinpub.ResolutionCacheKindSizeAttribute, sheinpub.SizeAttributeStoreID(req), key)
}

func mustMarshalSheinSizeAttributeReview(review *sheinpub.SizeAttributeReview) string {
	data, err := json.Marshal(review)
	if err != nil {
		return ""
	}
	return string(data)
}

func attachSizeAttributeCacheInfo(
	review *sheinpub.SizeAttributeReview,
	source string,
	key string,
	manual bool,
	hitSource string,
	status string,
	hitCount int,
	updatedAt *time.Time,
) {
	if review == nil {
		return
	}
	info := &sheinpub.ResolutionCacheInfo{
		Status:    pricingCacheStatusForSource(source, status),
		Source:    source,
		HitSource: hitSource,
		CacheKey:  key,
		ShortKey:  sheinpub.SizeAttributeShortKey(key),
		HitCount:  hitCount,
		Manual:    manual,
		Clearable: key != "",
	}
	if updatedAt != nil {
		copyUpdatedAt := *updatedAt
		info.UpdatedAt = &copyUpdatedAt
	}
	review.Cache = info
}

func sheinSizeAttributeCacheSkipReason(pkg *sheinpub.Package) string {
	switch {
	case pkg == nil:
		return "package_nil"
	case pkg.SizeAttributes != nil && pkg.SizeAttributes.Ready:
		return "existing_ready_size_attributes"
	case pkg.DraftPayload == nil || len(pkg.DraftPayload.SizeAttributeList) == 0:
		return "empty_size_attribute_list"
	default:
		return ""
	}
}

func sheinSizeAttributeCacheInfo(pkg *sheinpub.Package) *sheinpub.ResolutionCacheInfo {
	if pkg == nil || pkg.SizeAttributes == nil {
		return nil
	}
	return pkg.SizeAttributes.Cache
}

func logSizeAttributeCacheEvent(event string, req *sheinpub.BuildRequest, pkg *sheinpub.Package, info *sheinpub.ResolutionCacheInfo, fields logrus.Fields) {
	log := logrus.WithFields(logrus.Fields{
		"component": "listingkit/size_attribute_cache",
		"event":     event,
		"store_id":  sheinpub.SizeAttributeStoreID(req),
		"category_id": func() int {
			if pkg == nil {
				return 0
			}
			return pkg.CategoryID
		}(),
	})
	for key, value := range fields {
		log = log.WithField(key, value)
	}
	if info != nil {
		log = log.WithFields(logrus.Fields{
			"cache_source": info.Source,
			"cache_key":    info.CacheKey,
			"short_key":    info.ShortKey,
		})
	}
	log.Info("processed SHEIN size attribute cache")
}
