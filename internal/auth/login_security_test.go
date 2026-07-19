package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleLoginRejectsUnsupportedAndOversizedRequests(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	unsupported := httptest.NewRequest(http.MethodPut, "/login", nil)
	unsupportedResponse := httptest.NewRecorder()
	HandleLogin(unsupportedResponse, unsupported, db)
	if unsupportedResponse.Code != http.StatusMethodNotAllowed || unsupportedResponse.Header().Get("Allow") != "GET, POST" {
		t.Fatalf("unsupported response = %d Allow=%q", unsupportedResponse.Code, unsupportedResponse.Header().Get("Allow"))
	}

	oversized := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username="+strings.Repeat("x", 70<<10)))
	oversized.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	oversizedResponse := httptest.NewRecorder()
	HandleLogin(oversizedResponse, oversized, db)
	if oversizedResponse.Code != http.StatusBadRequest {
		t.Fatalf("oversized response = %d, want 400", oversizedResponse.Code)
	}
}

func TestHandleLoginSetsHardenedCookie(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	insertTestUser(t, db, "tls-user", "loginpass", 0)

	form := url.Values{"username": {"tls-user"}, "password": {"loginpass"}}
	req := httptest.NewRequest(http.MethodPost, "https://example.test/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	HandleLogin(response, req, db)
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %+v", cookies)
	}
	cookie := cookies[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 24*60*60 {
		t.Fatalf("login cookie is not fully hardened: %+v", cookie)
	}
}
