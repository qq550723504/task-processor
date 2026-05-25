package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestSensitiveWordHandlerListsWordsWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newSensitiveWordTestRouter(t)
	seedSensitiveWord(t, router.db, listingSensitiveWord{
		TenantID:    101,
		Word:        "restricted",
		Language:    "en",
		Tags:        "policy",
		Level:       2,
		ReplaceWord: "safe",
		Status:      1,
	})
	seedSensitiveWord(t, router.db, listingSensitiveWord{
		TenantID: 202,
		Word:     "other",
		Language: "en",
		Level:    1,
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodGet, "/sensitive-words?page=1&page_size=20&language=en", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /sensitive-words = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page SensitiveWordPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one word", page)
	}
	if page.Items[0].Word != "restricted" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 word only", page.Items)
	}
}

func TestSensitiveWordHandlerRejectsInvalidNumericFilters(t *testing.T) {
	t.Parallel()

	router := newSensitiveWordTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/sensitive-words?level=abc", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /sensitive-words invalid filter = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_level"`) {
		t.Fatalf("response body = %s, want invalid_level", resp.Body.String())
	}
}

func TestSensitiveWordHandlerCreatesWordWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newSensitiveWordTestRouter(t)
	body := bytes.NewBufferString(`{
		"word":"restricted",
		"language":"en",
		"tags":"policy,brand",
		"level":3,
		"replaceWord":"safe",
		"status":1,
		"remark":"manual"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/sensitive-words", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /sensitive-words = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created SensitiveWord
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.Word != "restricted" || created.Language != "en" {
		t.Fatalf("created = %+v, want tenant scoped sensitive word", created)
	}
}

func TestSensitiveWordHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newSensitiveWordTestRouter(t)
	word := seedSensitiveWord(t, router.db, listingSensitiveWord{
		TenantID: 505,
		Word:     "restricted",
		Language: "en",
		Level:    1,
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/sensitive-words/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /sensitive-words/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingSensitiveWord
	if err := router.db.Table("listing_sensitive_word").Where("id = ?", word.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func newSensitiveWordTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingSensitiveWord{}); err != nil {
		t.Fatalf("migrate listing_sensitive_word: %v", err)
	}
	repo := NewGormSensitiveWordRepository(router.db)
	handler := NewSensitiveWordHandler(repo)
	router.engine.GET("/sensitive-words", handler.ListSensitiveWords)
	router.engine.POST("/sensitive-words", handler.CreateSensitiveWord)
	router.engine.DELETE("/sensitive-words/:id", handler.DeleteSensitiveWord)
	return router
}

func seedSensitiveWord(t *testing.T, db *gorm.DB, word listingSensitiveWord) listingSensitiveWord {
	t.Helper()
	if err := db.Table("listing_sensitive_word").Create(&word).Error; err != nil {
		t.Fatalf("seed sensitive word: %v", err)
	}
	return word
}
