package listingkit

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	sheinpub "task-processor/internal/publishing/shein"
	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

func cloneSheinProductForSubmit(product *sheinproduct.Product) (*sheinproduct.Product, error) {
	if product == nil {
		return nil, nil
	}
	data, err := json.Marshal(product)
	if err != nil {
		return nil, fmt.Errorf("clone shein product: %w", err)
	}
	var cloned sheinproduct.Product
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("clone shein product: %w", err)
	}
	return &cloned, nil
}

func sheinProductImageURLCount(product *sheinproduct.Product) int {
	if product == nil {
		return 0
	}
	count := sheinImageInfoURLCount(product.ImageInfo)
	for i := range product.SKCList {
		count += sheinImageInfoURLCount(&product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			count += sheinImageInfoURLCount(product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return count
}

func sheinProductPendingImageUploadCount(product *sheinproduct.Product) int {
	if product == nil {
		return 0
	}
	count := sheinImageInfoPendingUploadCount(product.ImageInfo)
	for i := range product.SKCList {
		count += sheinImageInfoPendingUploadCount(&product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			count += sheinImageInfoPendingUploadCount(product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return count
}

func sheinImageInfoURLCount(info *sheinproduct.ImageInfo) int {
	if info == nil {
		return 0
	}
	count := 0
	for _, image := range info.ImageInfoList {
		if strings.TrimSpace(image.ImageURL) != "" {
			count++
		}
	}
	return count
}

func sheinImageInfoPendingUploadCount(info *sheinproduct.ImageInfo) int {
	if info == nil {
		return 0
	}
	count := 0
	for _, image := range info.ImageInfoList {
		url := strings.TrimSpace(image.ImageURL)
		if url != "" && !isSheinUploadedImageURL(url) {
			count++
		}
	}
	return count
}

func uploadSheinProductImages(product *sheinproduct.Product, uploader sheinimage.ImageAPI, cached map[string]string) (int, map[string]string, error) {
	if product == nil {
		return 0, cloneSheinImageUploadCache(cached), nil
	}
	if uploader == nil {
		return 0, cloneSheinImageUploadCache(cached), fmt.Errorf("shein image upload api is not configured")
	}
	uploaded := cloneSheinImageUploadCache(cached)
	refs := collectSheinProductImageRefs(product)
	pending := map[string]sheinImageUploadJob{}
	for _, ref := range refs {
		uploadedURL, ok := uploaded[ref.cacheKey]
		if ok && !isSheinUploadedImageURL(uploadedURL) {
			ok = false
		}
		if ok {
			continue
		}
		if _, exists := pending[ref.cacheKey]; exists {
			continue
		}
		pending[ref.cacheKey] = sheinImageUploadJob{
			cacheKey:     ref.cacheKey,
			sourceURL:    ref.sourceURL,
			isColorBlock: ref.isColorBlock,
		}
	}
	count, err := uploadSheinImageJobs(pending, uploader, uploaded)
	if err != nil {
		return count, uploaded, err
	}
	for _, ref := range refs {
		if uploadedURL := strings.TrimSpace(uploaded[ref.cacheKey]); isSheinUploadedImageURL(uploadedURL) {
			ref.image.ImageURL = uploadedURL
		}
	}
	return count, uploaded, nil
}

const sheinSubmitImageUploadConcurrency = 3

type sheinImageUploadRef struct {
	image        *sheinproduct.ImageDetail
	sourceURL    string
	cacheKey     string
	isColorBlock bool
}

type sheinImageUploadJob struct {
	cacheKey     string
	sourceURL    string
	isColorBlock bool
}

type sheinImageUploadResult struct {
	cacheKey    string
	uploadedURL string
	err         error
}

func collectSheinProductImageRefs(product *sheinproduct.Product) []sheinImageUploadRef {
	if product == nil {
		return nil
	}
	refs := make([]sheinImageUploadRef, 0, sheinProductImageURLCount(product))
	refs = appendSheinImageInfoRefs(refs, product.ImageInfo)
	for i := range product.SKCList {
		refs = appendSheinImageInfoRefs(refs, &product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			refs = appendSheinImageInfoRefs(refs, product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return refs
}

func appendSheinImageInfoRefs(refs []sheinImageUploadRef, info *sheinproduct.ImageInfo) []sheinImageUploadRef {
	if info == nil {
		return refs
	}
	for i := range info.ImageInfoList {
		sourceURL := strings.TrimSpace(info.ImageInfoList[i].ImageURL)
		if sourceURL == "" || isSheinUploadedImageURL(sourceURL) {
			continue
		}
		isColorBlock := info.ImageInfoList[i].ImageType == 6 && !info.ImageInfoList[i].SizeImgFlag
		cacheKey := sourceURL
		if isColorBlock {
			cacheKey = "color-block:" + sourceURL
		}
		refs = append(refs, sheinImageUploadRef{
			image:        &info.ImageInfoList[i],
			sourceURL:    sourceURL,
			cacheKey:     cacheKey,
			isColorBlock: isColorBlock,
		})
	}
	return refs
}

func isSheinUploadedImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "shein.com") ||
		strings.Contains(value, "sheinimg.com") ||
		strings.Contains(value, "ltwebstatic.com")
}

func isSDSImageURL(url string) bool {
	value := strings.ToLower(strings.TrimSpace(url))
	return strings.Contains(value, "sdspod.com") || strings.Contains(value, "sdsdiy.com")
}

func uploadSheinImageJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}
	normalJobs := make(map[string]sheinImageUploadJob, len(jobs))
	colorBlockJobs := make(map[string]sheinImageUploadJob)
	for key, job := range jobs {
		if job.isColorBlock {
			colorBlockJobs[key] = job
			continue
		}
		normalJobs[key] = job
	}
	count := 0
	if len(normalJobs) > 0 {
		added, err := runSheinImageUploadJobs(normalJobs, uploader, uploaded, cloneSheinImageUploadCache(uploaded))
		count += added
		if err != nil {
			return count, err
		}
	}
	if len(colorBlockJobs) > 0 {
		added, err := runSheinImageUploadJobs(colorBlockJobs, uploader, uploaded, cloneSheinImageUploadCache(uploaded))
		count += added
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func runSheinImageUploadJobs(jobs map[string]sheinImageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string, existing map[string]string) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}
	work := make(chan sheinImageUploadJob)
	results := make(chan sheinImageUploadResult, len(jobs))
	workerCount := sheinSubmitImageUploadConcurrency
	if len(jobs) < workerCount {
		workerCount = len(jobs)
	}
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range work {
				uploadedURL, err := uploadSingleSheinImage(job, uploader, existing)
				results <- sheinImageUploadResult{
					cacheKey:    job.cacheKey,
					uploadedURL: uploadedURL,
					err:         err,
				}
			}
		}()
	}
	go func() {
		for _, job := range jobs {
			work <- job
		}
		close(work)
		wg.Wait()
		close(results)
	}()

	count := 0
	for result := range results {
		if result.err != nil {
			return count, result.err
		}
		uploaded[result.cacheKey] = result.uploadedURL
		count++
	}
	return count, nil
}

func uploadSingleSheinImage(job sheinImageUploadJob, uploader sheinimage.ImageAPI, existing map[string]string) (string, error) {
	if job.isColorBlock {
		imageData, err := buildSheinColorBlockImageFromURL(job.sourceURL)
		if err == nil {
			uploadedURL, uploadErr := uploader.UploadOriginalImage(imageData)
			if uploadErr == nil {
				return uploadedURL, nil
			}
			err = uploadErr
		}
		if existingURL := strings.TrimSpace(existing[job.sourceURL]); isSheinUploadedImageURL(existingURL) {
			return existingURL, nil
		}
		uploadedURL, uploadErr := uploader.DownloadAndUploadImage(job.sourceURL)
		if uploadErr != nil {
			return "", fmt.Errorf("upload shein image %q: %w", job.sourceURL, uploadErr)
		}
		return uploadedURL, nil
	}
	uploadedURL, err := uploader.DownloadAndUploadImage(job.sourceURL)
	if err != nil {
		return "", fmt.Errorf("upload shein image %q: %w", job.sourceURL, err)
	}
	return uploadedURL, nil
}

func uploadSheinImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string) (int, error) {
	if info == nil {
		return 0, nil
	}
	count := 0
	for i := range info.ImageInfoList {
		sourceURL := strings.TrimSpace(info.ImageInfoList[i].ImageURL)
		if sourceURL == "" {
			continue
		}
		isColorBlock := info.ImageInfoList[i].ImageType == 6 && !info.ImageInfoList[i].SizeImgFlag
		if isSheinUploadedImageURL(sourceURL) {
			continue
		}
		cacheKey := sourceURL
		if isColorBlock {
			cacheKey = "color-block:" + sourceURL
		}
		uploadedURL, ok := uploaded[cacheKey]
		if ok && !isSheinUploadedImageURL(uploadedURL) {
			ok = false
		}
		if !ok {
			var err error
			if isColorBlock {
				var imageData []byte
				imageData, err = buildSheinColorBlockImageFromURL(sourceURL)
				if err == nil {
					uploadedURL, err = uploader.UploadOriginalImage(imageData)
				}
				if err != nil {
					if existingURL := strings.TrimSpace(uploaded[sourceURL]); isSheinUploadedImageURL(existingURL) {
						uploadedURL = existingURL
						err = nil
					} else {
						uploadedURL, err = uploader.DownloadAndUploadImage(sourceURL)
					}
				}
			} else {
				uploadedURL, err = uploader.DownloadAndUploadImage(sourceURL)
			}
			if err != nil {
				return count, fmt.Errorf("upload shein image %q: %w", sourceURL, err)
			}
			uploaded[cacheKey] = uploadedURL
			count++
		}
		info.ImageInfoList[i].ImageURL = uploadedURL
	}
	return count, nil
}

func sheinImageUploadCache(pkg *SheinPackage) map[string]string {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return nil
	}
	return pkg.FinalSubmissionDraft.SheinImageUploadCache
}

func sheinImageUploadCacheHit(pkg *SheinPackage, sourceURL string) bool {
	uploadedURL := strings.TrimSpace(sheinImageUploadCache(pkg)[strings.TrimSpace(sourceURL)])
	return uploadedURL != "" && isSheinUploadedImageURL(uploadedURL)
}

func cloneSheinImageUploadCache(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for sourceURL, uploadedURL := range input {
		sourceURL = strings.TrimSpace(sourceURL)
		uploadedURL = strings.TrimSpace(uploadedURL)
		if sourceURL == "" || uploadedURL == "" || !isSheinUploadedImageURL(uploadedURL) {
			continue
		}
		out[sourceURL] = uploadedURL
	}
	return out
}
