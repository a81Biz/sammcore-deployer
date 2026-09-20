package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"k8s.io/client-go/kubernetes/fake"

	"sammcore-deployer/services"
	"sammcore-deployer/storage"
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

func TestDeployAndManageEndpoints(t *testing.T) {
	apiKey := "super-secret-key-123"
	os.Setenv("DEPLOYER_API_KEY", apiKey)
	defer os.Unsetenv("DEPLOYER_API_KEY")

	tmpDir, err := os.MkdirTemp("", "test-api-store-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	// Usar mock deploy manager con fake clientset
	fakeClient := fake.NewSimpleClientset()
	fakeDM := services.NewDeployManager(fakeClient)
	SetDeployManager(fakeDM)

	r := NewRouter()

	// 1. POST /api/deploy (debe responder 202 Accepted)
	deployBody := []byte(`{
		"name": "backroom",
		"repo": "https://github.com/a81Biz/backroom",
		"branch": "master",
		"type": "compose",
		"web_port": 80,
		"api_port": 8000
	}`)

	req, _ := http.NewRequest("POST", "/api/deploy", bytes.NewReader(deployBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on /api/deploy, got %v (body: %s)", rr.Code, rr.Body.String())
	}

	// 2. GET /api/projects/:id (debe responder 200)
	projectID := "a81biz-backroom"
	req, _ = http.NewRequest("GET", "/api/projects/"+projectID, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr = httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /api/projects/%s, got %v", projectID, rr.Code)
	}

	// 3. GET /api/projects/:id/logs (debe responder 200)
	req, _ = http.NewRequest("GET", "/api/projects/"+projectID+"/logs", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr = httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET logs, got %v", rr.Code)
	}

	// 4. POST /api/projects/:id/redeploy (debe responder 202 Accepted)
	req, _ = http.NewRequest("POST", "/api/projects/"+projectID+"/redeploy", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr = httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on redeploy, got %v", rr.Code)
	}

	// 5. DELETE /api/projects/:id (debe responder 200 OK)
	req, _ = http.NewRequest("DELETE", "/api/projects/"+projectID+"?delete_db=false", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	rr = httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete project, got %v", rr.Code)
	}

	// 6. Verificar que ya no existe
	_, err = storage.GetProject(projectID)
	if err == nil {
		t.Errorf("expected project to be deleted from storage")
	}
}
