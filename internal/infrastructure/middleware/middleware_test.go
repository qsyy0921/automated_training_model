package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayAuthAllowsLoopbackWithoutToken(t *testing.T) {
	handler := GatewayAuth(GatewayAuthConfig{})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected loopback request to pass, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGatewayAuthRejectsRemoteWithoutConfiguredToken(t *testing.T) {
	handler := GatewayAuth(GatewayAuthConfig{})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	req.RemoteAddr = "192.168.1.20:54321"
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected remote request without configured token to be forbidden, got %d", resp.Code)
	}
}

func TestGatewayAuthAllowsRemoteBearerToken(t *testing.T) {
	handler := GatewayAuth(GatewayAuthConfig{Token: "secret"})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	req.RemoteAddr = "192.168.1.20:54321"
	req.Header.Set("Authorization", "Bearer secret")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected bearer token to pass, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestGatewayAuthCanRequireTokenForLoopback(t *testing.T) {
	handler := GatewayAuth(GatewayAuthConfig{Token: "secret", RequireTokenForLoopback: true})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/api/runtime/status", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected loopback request without token to be unauthorized, got %d", resp.Code)
	}
}

func TestGatewayAuthKeepsHealthPublic(t *testing.T) {
	handler := GatewayAuth(GatewayAuthConfig{RequireTokenForLoopback: true})(okHandler())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.RemoteAddr = "192.168.1.20:54321"
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected health endpoint to stay public, got %d", resp.Code)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
