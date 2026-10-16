package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestAPI(t *testing.T) (*gin.Engine, *store, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	accounts := newStore()
	admin, err := accounts.createInitialAdmin("admin@example.com", "correct-horse-battery")
	if err != nil {
		t.Fatalf("create initial admin: %v", err)
	}
	_, adminToken, err := accounts.login(admin.Email, "correct-horse-battery")
	if err != nil {
		t.Fatalf("login initial admin: %v", err)
	}

	audit, err := openAuditLog(filepath.Join(t.TempDir(), "audit.log"), bytes.Repeat([]byte("k"), 32))
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	t.Cleanup(func() {
		if err := audit.Close(); err != nil {
			t.Errorf("close audit log: %v", err)
		}
	})

	router := gin.New()
	router.Use(audit.middleware(), gin.Recovery())
	registerRoutes(router, accounts)
	return router, accounts, adminToken
}

func performRequest(router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	var encoded bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&encoded).Encode(body)
	}
	request := httptest.NewRequest(method, path, &encoded)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response body %q: %v", response.Body.String(), err)
	}
	return result
}

func TestVehicleLifecycleAndRoleAuthorization(t *testing.T) {
	router, _, adminToken := newTestAPI(t)

	response := performRequest(router, http.MethodPost, "/api/users", "", map[string]any{
		"name": "Ana Pérez", "email": "ana@example.com",
		"phone": "+57 300 000 0000", "password": "ana-secure-password",
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("register seller: got %d, want %d: %s", response.Code, http.StatusCreated, response.Body)
	}
	seller := decodeResponse[userResponse](t, response)

	response = performRequest(router, http.MethodPost, "/api/sessions", "", map[string]string{
		"email": "ana@example.com", "password": "ana-secure-password",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("login seller: got %d, want %d: %s", response.Code, http.StatusOK, response.Body)
	}
	var loginResult struct {
		AccessToken string `json:"access_token"`
	}
	loginResult = decodeResponse[struct {
		AccessToken string `json:"access_token"`
	}](t, response)
	if loginResult.AccessToken == "" {
		t.Fatal("login response omitted access token")
	}
	sellerToken := loginResult.AccessToken

	response = performRequest(router, http.MethodPost, "/api/vehicles", "", vehicleInput{
		Title: "Toyota Corolla 2022", Make: "Toyota", Model: "Corolla",
		Year: 2022, PriceCents: 8500000000, Description: "Mantenimiento al día.",
	})
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated vehicle create: got %d, want %d", response.Code, http.StatusUnauthorized)
	}

	response = performRequest(router, http.MethodPost, "/api/vehicles", sellerToken, vehicleInput{
		Title: "Toyota Corolla 2022", Make: "Toyota", Model: "Corolla",
		Year: 2022, PriceCents: 8500000000, Description: "Mantenimiento al día.",
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("create vehicle: got %d, want %d: %s", response.Code, http.StatusCreated, response.Body)
	}
	item := decodeResponse[vehicle](t, response)
	if item.State != statePending || item.SellerID != seller.ID {
		t.Fatalf("new vehicle has state %q and seller %q", item.State, item.SellerID)
	}

	response = performRequest(router, http.MethodGet, "/api/vehicles", "", nil)
	publicListing := decodeResponse[struct {
		Vehicles []publicVehicle `json:"vehicles"`
	}](t, response)
	if len(publicListing.Vehicles) != 0 {
		t.Fatalf("pending vehicle leaked to public catalog: got %d entries", len(publicListing.Vehicles))
	}

	response = performRequest(router, http.MethodGet, "/api/admin/vehicles/pending", sellerToken, nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("seller accessing admin endpoint: got %d, want %d", response.Code, http.StatusForbidden)
	}

	response = performRequest(router, http.MethodPost, "/api/admin/vehicles/"+item.ID+"/approve", adminToken, nil)
	if response.Code != http.StatusNoContent {
		t.Fatalf("approve vehicle: got %d, want %d: %s", response.Code, http.StatusNoContent, response.Body)
	}

	response = performRequest(router, http.MethodGet, "/api/vehicles/"+item.ID, "", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("get public vehicle: got %d, want %d: %s", response.Code, http.StatusOK, response.Body)
	}
	published := decodeResponse[publicVehicle](t, response)
	if published.Seller.Email != "ana@example.com" || published.Seller.Phone != "+57 300 000 0000" {
		t.Fatalf("public vehicle omitted seller contact details: %+v", published.Seller)
	}

	response = performRequest(router, http.MethodPost, "/api/vehicles/"+item.ID+"/sold", sellerToken, nil)
	if response.Code != http.StatusNoContent {
		t.Fatalf("report vehicle sold: got %d, want %d: %s", response.Code, http.StatusNoContent, response.Body)
	}
	response = performRequest(router, http.MethodGet, "/api/vehicles/"+item.ID, "", nil)
	if response.Code != http.StatusNotFound {
		t.Fatalf("sold vehicle remained public: got %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRegistrationRejectsClientSuppliedRole(t *testing.T) {
	router, _, _ := newTestAPI(t)
	response := performRequest(router, http.MethodPost, "/api/users", "", map[string]any{
		"name": "Attacker", "email": "attacker@example.com",
		"phone": "+1 555 0100", "password": "attacker-secure-password", "role": "admin",
	})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("role assignment attempt: got %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestWebInterfaceAndRoleSpecificEndpoints(t *testing.T) {
	router, _, adminToken := newTestAPI(t)

	response := performRequest(router, http.MethodGet, "/", "", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "AutoMarket") {
		t.Fatalf("web interface: got %d: %s", response.Code, response.Body)
	}
	response = performRequest(router, http.MethodGet, "/static/app.js", "", nil)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "/api/vehicles") {
		t.Fatalf("web application script: got %d", response.Code)
	}

	response = performRequest(router, http.MethodPost, "/api/users", "", map[string]any{
		"name": "Ana Pérez", "email": "ana@example.com",
		"phone": "+57 300 000 0000", "password": "ana-secure-password",
	})
	if response.Code != http.StatusCreated {
		t.Fatalf("register seller: got %d: %s", response.Code, response.Body)
	}
	response = performRequest(router, http.MethodPost, "/api/sessions", "", map[string]string{
		"email": "ana@example.com", "password": "ana-secure-password",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("login seller: got %d: %s", response.Code, response.Body)
	}
	var loginResult struct {
		AccessToken string `json:"access_token"`
	}
	loginResult = decodeResponse[struct {
		AccessToken string `json:"access_token"`
	}](t, response)
	sellerToken := loginResult.AccessToken

	response = performRequest(router, http.MethodGet, "/api/me", sellerToken, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("get current seller: got %d: %s", response.Code, response.Body)
	}
	if account := decodeResponse[userResponse](t, response); account.Email != "ana@example.com" {
		t.Fatalf("current user endpoint returned unexpected account: %+v", account)
	}

	response = performRequest(router, http.MethodGet, "/api/my/vehicles", sellerToken, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("list seller vehicles: got %d: %s", response.Code, response.Body)
	}
	sellerVehicles := decodeResponse[struct {
		Vehicles []vehicle `json:"vehicles"`
	}](t, response)
	if len(sellerVehicles.Vehicles) != 0 {
		t.Fatalf("new seller should have no vehicles, got %d", len(sellerVehicles.Vehicles))
	}

	response = performRequest(router, http.MethodGet, "/api/admin/users", sellerToken, nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("seller accessing user administration: got %d, want %d", response.Code, http.StatusForbidden)
	}
	response = performRequest(router, http.MethodGet, "/api/admin/vehicles", sellerToken, nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("seller accessing all vehicle administration: got %d, want %d", response.Code, http.StatusForbidden)
	}
	response = performRequest(router, http.MethodGet, "/api/admin/vehicles", adminToken, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("admin listing all vehicles: got %d: %s", response.Code, response.Body)
	}
	response = performRequest(router, http.MethodGet, "/api/admin/users", adminToken, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("admin listing users: got %d: %s", response.Code, response.Body)
	}
	users := decodeResponse[struct {
		Users []userResponse `json:"users"`
	}](t, response)
	if len(users.Users) != 2 {
		t.Fatalf("admin should see both accounts, got %d", len(users.Users))
	}
}

func TestAuditLogDetectsTampering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	key := bytes.Repeat([]byte("k"), 32)
	audit, err := openAuditLog(path, key)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	if err := audit.append(auditPayload{Method: http.MethodGet, Path: "/api/vehicles", Status: http.StatusOK}); err != nil {
		t.Fatalf("append audit record: %v", err)
	}
	if err := audit.Close(); err != nil {
		t.Fatalf("close audit log: %v", err)
	}

	audit, err = openAuditLog(path, key)
	if err != nil {
		t.Fatalf("verify valid audit log on reopen: %v", err)
	}
	if err := audit.Close(); err != nil {
		t.Fatalf("close reopened audit log: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	tampered := strings.Replace(string(data), "/api/vehicles", "/api/attacked", 1)
	if tampered == string(data) {
		t.Fatal("test setup failed to locate audit path")
	}
	if err := os.WriteFile(path, []byte(tampered), 0600); err != nil {
		t.Fatalf("tamper with audit log: %v", err)
	}
	if _, err := openAuditLog(path, key); err == nil {
		t.Fatal("expected tampered audit log to fail verification")
	}
}
