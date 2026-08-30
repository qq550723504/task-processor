package local

import "context"

func (r *LocalRuntime) GetSheinCookie(storeID int64) (string, int64, error) {
	if r == nil || r.cookieProvider == nil {
		return "", 0, nil
	}
	result, err := r.cookieProvider.GetCookie(context.Background(), storeID)
	if err != nil || result == nil {
		return "", 0, err
	}
	return result.CookieJSON, result.TenantID, nil
}

func (r *LocalRuntime) GetSheinStoreCookie(storeID int64) (string, error) {
	cookie, _, err := r.GetSheinCookie(storeID)
	return cookie, err
}

func (r *LocalRuntime) DeleteSheinStoreCookie(storeID int64) (bool, error) {
	if r == nil || r.cookieProvider == nil {
		return false, nil
	}
	return r.cookieProvider.DeleteCookie(context.Background(), storeID)
}
