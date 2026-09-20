package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	r := NewRouter()
	req, _ := http.NewRequest("GET", "/api/health", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func TestAuthMiddleware_Unauthorized(t *testing.T) {
	os.Setenv("DEPLOYER_API_KEY", "super-secret-key-123")
	defer os.Unsetenv("DEPLOYER_API_KEY")

	r := NewRouter()
	req, _ := http.NewRequest("GET", "/api/projects", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %v", status)
	}

	var errResp APIError
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Status != "error" || errResp.Code != "UNAUTHORIZED_HEADER" {
		t.Errorf("expected status=error, code=UNAUTHORIZED_HEADER, got %v", errResp)
	}
}

func TestAuthMiddleware_Authorized(t *testing.T) {
	apiKey := "super-secret-key-123"
	os.Setenv("DEPLOYER_API_KEY", apiKey)
	defer os.Unsetenv("DEPLOYER_API_KEY")

	r := NewRouter()
	req, _ := http.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected 200 OK, got %v", status)
	}
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	r := NewRouter()
	req, _ := http.NewRequest("OPTIONS", "/api/health", nil)
	req.Header.Set("Origin", "https://malicious-site.com")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for disallowed CORS origin, got %v", status)
	}
}

func TestNotImplementedEndpoints(t *testing.T) {
	apiKey := "super-secret-key-123"
	os.Setenv("DEPLOYER_API_KEY", apiKey)
	defer os.Unsetenv("DEPLOYER_API_KEY")

	r := NewRouter()

	endpoints := []struct {
		method string
		path   string
	}{
		{"POST", "/api/deploy"},
		{"DELETE", "/api/projects/test-proj"},
		{"GET", "/api/projects/test-proj/logs"},
		{"POST", "/api/projects/test-proj/redeploy"},
	}

	for _, ep := range endpoints {
		req, _ := http.NewRequest(ep.method, ep.path, nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotImplemented {
			t.Errorf("[%s %s] expected 501 Not Implemented, got %v", ep.method, ep.path, rr.Code)
		}

		var errResp APIError
		if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
			t.Errorf("[%s %s] failed to decode error response: %v", ep.method, ep.path, err)
		}
		if errResp.Status != "error" || errResp.Code != "NOT_IMPLEMENTED" {
			t.Errorf("[%s %s] expected code=NOT_IMPLEMENTED, got %v", ep.method, ep.path, errResp.Code)
		}
	}
}
