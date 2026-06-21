package shein

import (
	"fmt"
	"strings"
	"sync"

	sheinimage "task-processor/internal/shein/api/image"
	sheinproduct "task-processor/internal/shein/api/product"
)

const defaultImageUploadConcurrency = 3

// ColorBlockBuilder builds uploadable color-block image bytes from a source image URL.
type ColorBlockBuilder func(imageURL string) ([]byte, error)

type imageUploadRef struct {
	image        *sheinproduct.ImageDetail
	sourceURL    string
	cacheKey     string
	isColorBlock bool
}

type imageUploadJob struct {
	cacheKey     string
	sourceURL    string
	isColorBlock bool
}

type imageUploadResult struct {
	cacheKey    string
	uploadedURL string
	err         error
}

// UploadProductImages uploads all pending SHEIN product images and rewrites payload URLs.
func UploadProductImages(product *sheinproduct.Product, uploader sheinimage.ImageAPI, cached map[string]string, colorBlockBuilder ColorBlockBuilder) (int, map[string]string, error) {
	if product == nil {
		return 0, CloneImageUploadCache(cached), nil
	}
	if uploader == nil {
		return 0, CloneImageUploadCache(cached), fmt.Errorf("shein image upload api is not configured")
	}
	uploaded := CloneImageUploadCache(cached)
	refs := collectProductImageRefs(product)
	pending := map[string]imageUploadJob{}
	for _, ref := range refs {
		uploadedURL, ok := uploaded[ref.cacheKey]
		if ok && !IsUploadedImageURL(uploadedURL) {
			ok = false
		}
		if ok {
			continue
		}
		if _, exists := pending[ref.cacheKey]; exists {
			continue
		}
		pending[ref.cacheKey] = imageUploadJob{
			cacheKey:     ref.cacheKey,
			sourceURL:    ref.sourceURL,
			isColorBlock: ref.isColorBlock,
		}
	}
	count, err := uploadImageJobs(pending, uploader, uploaded, colorBlockBuilder)
	if err != nil {
		return count, uploaded, err
	}
	for _, ref := range refs {
		if uploadedURL := strings.TrimSpace(uploaded[ref.cacheKey]); IsUploadedImageURL(uploadedURL) {
			ref.image.ImageURL = uploadedURL
		}
	}
	return count, uploaded, nil
}

// UploadImageInfo uploads pending images for one image info object and rewrites payload URLs.
func UploadImageInfo(info *sheinproduct.ImageInfo, uploader sheinimage.ImageAPI, uploaded map[string]string, colorBlockBuilder ColorBlockBuilder) (int, error) {
	if info == nil {
		return 0, nil
	}
	count := 0
	for i := range info.ImageInfoList {
		sourceURL := strings.TrimSpace(info.ImageInfoList[i].ImageURL)
		if sourceURL == "" {
			continue
		}
		isColorBlock := isColorBlockImage(info.ImageInfoList[i])
		if IsUploadedImageURL(sourceURL) {
			continue
		}
		cacheKey := imageUploadCacheKey(sourceURL, isColorBlock)
		uploadedURL, ok := uploaded[cacheKey]
		if ok && !IsUploadedImageURL(uploadedURL) {
			ok = false
		}
		if !ok {
			var err error
			uploadedURL, err = uploadImageURL(sourceURL, isColorBlock, uploader, uploaded, colorBlockBuilder)
			if err != nil {
				return count, err
			}
			uploaded[cacheKey] = uploadedURL
			count++
		}
		info.ImageInfoList[i].ImageURL = uploadedURL
	}
	return count, nil
}

func collectProductImageRefs(product *sheinproduct.Product) []imageUploadRef {
	if product == nil {
		return nil
	}
	refs := make([]imageUploadRef, 0, ProductImageURLCount(product))
	refs = appendImageInfoRefs(refs, product.ImageInfo)
	for i := range product.SKCList {
		refs = appendImageInfoRefs(refs, &product.SKCList[i].ImageInfo)
		for j := range product.SKCList[i].SKUS {
			refs = appendImageInfoRefs(refs, product.SKCList[i].SKUS[j].ImageInfo)
		}
	}
	return refs
}

func appendImageInfoRefs(refs []imageUploadRef, info *sheinproduct.ImageInfo) []imageUploadRef {
	if info == nil {
		return refs
	}
	for i := range info.ImageInfoList {
		sourceURL := strings.TrimSpace(info.ImageInfoList[i].ImageURL)
		if sourceURL == "" || IsUploadedImageURL(sourceURL) {
			continue
		}
		isColorBlock := isColorBlockImage(info.ImageInfoList[i])
		refs = append(refs, imageUploadRef{
			image:        &info.ImageInfoList[i],
			sourceURL:    sourceURL,
			cacheKey:     imageUploadCacheKey(sourceURL, isColorBlock),
			isColorBlock: isColorBlock,
		})
	}
	return refs
}

func uploadImageJobs(jobs map[string]imageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string, colorBlockBuilder ColorBlockBuilder) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}
	normalJobs := make(map[string]imageUploadJob, len(jobs))
	colorBlockJobs := make(map[string]imageUploadJob)
	for key, job := range jobs {
		if job.isColorBlock {
			colorBlockJobs[key] = job
			continue
		}
		normalJobs[key] = job
	}
	count := 0
	if len(normalJobs) > 0 {
		added, err := runImageUploadJobs(normalJobs, uploader, uploaded, CloneImageUploadCache(uploaded), colorBlockBuilder)
		count += added
		if err != nil {
			return count, err
		}
	}
	if len(colorBlockJobs) > 0 {
		added, err := runImageUploadJobs(colorBlockJobs, uploader, uploaded, CloneImageUploadCache(uploaded), colorBlockBuilder)
		count += added
		if err != nil {
			return count, err
		}
	}
	return count, nil
}

func runImageUploadJobs(jobs map[string]imageUploadJob, uploader sheinimage.ImageAPI, uploaded map[string]string, existing map[string]string, colorBlockBuilder ColorBlockBuilder) (int, error) {
	if len(jobs) == 0 {
		return 0, nil
	}
	work := make(chan imageUploadJob)
	results := make(chan imageUploadResult, len(jobs))
	workerCount := defaultImageUploadConcurrency
	if len(jobs) < workerCount {
		workerCount = len(jobs)
	}
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range work {
				uploadedURL, err := uploadSingleImage(job, uploader, existing, colorBlockBuilder)
				results <- imageUploadResult{
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

func uploadSingleImage(job imageUploadJob, uploader sheinimage.ImageAPI, existing map[string]string, colorBlockBuilder ColorBlockBuilder) (string, error) {
	return uploadImageURL(job.sourceURL, job.isColorBlock, uploader, existing, colorBlockBuilder)
}

func uploadImageURL(sourceURL string, isColorBlock bool, uploader sheinimage.ImageAPI, existing map[string]string, colorBlockBuilder ColorBlockBuilder) (string, error) {
	if isColorBlock {
		if colorBlockBuilder != nil {
			imageData, err := colorBlockBuilder(sourceURL)
			if err == nil {
				uploadedURL, uploadErr := uploader.UploadOriginalImage(imageData)
				if uploadErr == nil {
					return uploadedURL, nil
				}
			}
		}
		if existingURL := strings.TrimSpace(existing[sourceURL]); IsUploadedImageURL(existingURL) {
			return existingURL, nil
		}
		uploadedURL, uploadErr := uploader.DownloadAndUploadImage(sourceURL)
		if uploadErr != nil {
			return "", fmt.Errorf("upload shein image %q: %w", sourceURL, uploadErr)
		}
		return uploadedURL, nil
	}
	uploadedURL, err := uploader.DownloadAndUploadImage(sourceURL)
	if err != nil {
		return "", fmt.Errorf("upload shein image %q: %w", sourceURL, err)
	}
	return uploadedURL, nil
}

func isColorBlockImage(image sheinproduct.ImageDetail) bool {
	return image.ImageType == 6 && !image.SizeImgFlag
}

func imageUploadCacheKey(sourceURL string, isColorBlock bool) string {
	if isColorBlock {
		return "color-block:" + sourceURL
	}
	return sourceURL
}
