package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type authContextKey struct{}

type AuthContext struct {
	UserID   string `json:"userId"`
	Role     string `json:"role"`
	TenantID string `json:"tenantId"`
}

func authFromContext(ctx context.Context) AuthContext {
	auth, _ := ctx.Value(authContextKey{}).(AuthContext)
	return auth
}

func withAuth(ctx context.Context, auth AuthContext) context.Context {
	return context.WithValue(ctx, authContextKey{}, auth)
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		token := bearerToken(r)
		if token == "" {
			if cookie, err := r.Cookie("pawit_access"); err == nil {
				token = cookie.Value
			}
		}

		auth, err := s.validateToken(token)
		if err != nil {
			if s.cfg.AllowDevAuth && !s.cfg.IsProduction() {
				tenantID := strings.TrimSpace(r.Header.Get("X-PawIt-Tenant-ID"))
				if tenantID == "" {
					tenantID = "tenant_demo_clinic"
				}
				auth = AuthContext{UserID: "dev_user", Role: "SuperAdmin", TenantID: tenantID}
			} else {
				writeError(w, http.StatusUnauthorized, "authentication_required", "A valid PawIt access token is required.")
				return
			}
		}

		if auth.TenantID == "" {
			writeError(w, http.StatusBadRequest, "tenant_required", "Tenant scope is required for all application requests.")
			return
		}

		next.ServeHTTP(w, r.WithContext(withAuth(r.Context(), auth)))
	})
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if value == "" {
		return ""
	}
	prefix := "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}

func (s *Server) validateToken(token string) (AuthContext, error) {
	if token == "" {
		return AuthContext{}, errors.New("missing token")
	}
	if s.cfg.JWTSigningKey == "" {
		return AuthContext{}, errors.New("missing signing key")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AuthContext{}, errors.New("malformed token")
	}

	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSigningKey))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return AuthContext{}, errors.New("invalid signature")
	}

	var claims struct {
		Subject  string `json:"sub"`
		Role     string `json:"role"`
		TenantID string `json:"tenant_id"`
		Expires  int64  `json:"exp"`
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AuthContext{}, err
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return AuthContext{}, err
	}
	if claims.Expires <= time.Now().Unix() {
		return AuthContext{}, errors.New("token expired")
	}
	if claims.Subject == "" || claims.Role == "" || claims.TenantID == "" {
		return AuthContext{}, errors.New("required claims missing")
	}

	return AuthContext{UserID: claims.Subject, Role: claims.Role, TenantID: claims.TenantID}, nil
}
