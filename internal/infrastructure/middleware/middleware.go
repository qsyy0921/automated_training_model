package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

type Middleware func(http.Handler) http.Handler

type GatewayAuthConfig struct {
	Token                    string   `json:"-"`
	TokenConfigured          bool     `json:"token_configured"`
	RemoteRequiresToken      bool     `json:"remote_requires_token"`
	LoopbackBypass           bool     `json:"loopback_bypass"`
	RequireTokenForLoopback  bool     `json:"require_token_for_loopback"`
	AllowRemoteWithoutToken  bool     `json:"allow_remote_without_token"`
	ProtectedPathPrefixes    []string `json:"protected_path_prefixes"`
	PublicPathPrefixes       []string `json:"public_path_prefixes"`
	AllowedOriginsConfigured bool     `json:"allowed_origins_configured"`
	AllowedOrigins           []string `json:"allowed_origins,omitempty"`
}

func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = newRequestID()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), RequestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "panic", rec, "stack", string(debug.Stack()))
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func Logger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", r.Context().Value(RequestIDKey),
			)
		})
	}
}

func CORS() Middleware {
	return CORSWithOrigins(allowedOriginsFromEnv())
}

func CORSWithOrigins(allowedOrigins []string) Middleware {
	allowed := normalizeOrigins(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if len(allowed) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" && originAllowed(origin, allowed) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Gateway-Token,X-Request-ID")
			if r.Method == http.MethodOptions {
				if len(allowed) > 0 && origin != "" && !originAllowed(origin, allowed) {
					http.Error(w, "origin not allowed", http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func GatewayAuthFromEnv() Middleware {
	return GatewayAuth(GatewayAuthConfigFromEnv())
}

func GatewayAuthConfigFromEnv() GatewayAuthConfig {
	token := strings.TrimSpace(firstEnv("ATM_GATEWAY_TOKEN", "GATEWAY_AUTH_TOKEN"))
	requireLoopback := truthy(firstEnv("ATM_GATEWAY_REQUIRE_TOKEN_FOR_LOOPBACK", "GATEWAY_REQUIRE_TOKEN_FOR_LOOPBACK"))
	allowRemoteNoAuth := truthy(firstEnv("ATM_ALLOW_REMOTE_NO_AUTH", "GATEWAY_ALLOW_REMOTE_NO_AUTH"))
	allowedOrigins := allowedOriginsFromEnv()
	return GatewayAuthConfig{
		Token:                    token,
		TokenConfigured:          token != "",
		RemoteRequiresToken:      !allowRemoteNoAuth,
		LoopbackBypass:           !requireLoopback,
		RequireTokenForLoopback:  requireLoopback,
		AllowRemoteWithoutToken:  allowRemoteNoAuth,
		ProtectedPathPrefixes:    []string{"/api/"},
		PublicPathPrefixes:       []string{"/healthz", "/", "/assets/"},
		AllowedOriginsConfigured: len(allowedOrigins) > 0,
		AllowedOrigins:           normalizeOrigins(allowedOrigins),
	}
}

func GatewayAuthStatusFromEnv() GatewayAuthConfig {
	cfg := GatewayAuthConfigFromEnv()
	cfg.Token = ""
	return cfg
}

func GatewayAuth(cfg GatewayAuthConfig) Middleware {
	if len(cfg.ProtectedPathPrefixes) == 0 {
		cfg.ProtectedPathPrefixes = []string{"/api/"}
	}
	if len(cfg.PublicPathPrefixes) == 0 {
		cfg.PublicPathPrefixes = []string{"/healthz", "/", "/assets/"}
	}
	cfg.Token = strings.TrimSpace(cfg.Token)
	cfg.TokenConfigured = cfg.Token != ""
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !protectedPath(r.URL.Path, cfg.ProtectedPathPrefixes, cfg.PublicPathPrefixes) {
				next.ServeHTTP(w, r)
				return
			}
			if authorizedGatewayRequest(r, cfg) {
				next.ServeHTTP(w, r)
				return
			}
			writeAuthError(w, cfg, r)
		})
	}
}

func newRequestID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

func protectedPath(path string, protected []string, public []string) bool {
	for _, prefix := range public {
		if prefix == path {
			return false
		}
		if prefix != "/" && strings.HasSuffix(prefix, "/") && strings.HasPrefix(path, prefix) {
			return false
		}
	}
	for _, prefix := range protected {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func authorizedGatewayRequest(r *http.Request, cfg GatewayAuthConfig) bool {
	if cfg.Token != "" && bearerOrGatewayToken(r) == cfg.Token {
		return true
	}
	if isLoopbackRequest(r) && !cfg.RequireTokenForLoopback {
		return true
	}
	if cfg.Token == "" && cfg.AllowRemoteWithoutToken {
		return true
	}
	return false
}

func bearerOrGatewayToken(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Gateway-Token")); value != "" {
		return value
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("Bearer "):])
	}
	return ""
}

func writeAuthError(w http.ResponseWriter, cfg GatewayAuthConfig, r *http.Request) {
	status := http.StatusUnauthorized
	message := "gateway token required"
	if cfg.Token == "" && !isLoopbackRequest(r) && !cfg.AllowRemoteWithoutToken {
		status = http.StatusForbidden
		message = "gateway token must be configured before non-loopback access"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func isLoopbackRequest(r *http.Request) bool {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return strings.EqualFold(host, "localhost")
	}
	return ip.IsLoopback()
}

func originAllowed(origin string, allowed []string) bool {
	origin = strings.TrimSpace(origin)
	for _, item := range allowed {
		if item == "*" || subtle.ConstantTimeCompare([]byte(origin), []byte(item)) == 1 {
			return true
		}
	}
	return false
}

func allowedOriginsFromEnv() []string {
	raw := firstEnv("ATM_ALLOWED_ORIGINS", "GATEWAY_ALLOWED_ORIGINS")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func normalizeOrigins(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
