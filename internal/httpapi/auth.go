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
		if publicAuthPath(r.URL.Path) {
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
				userID := strings.TrimSpace(r.Header.Get("X-PawIt-User-ID"))
				if userID == "" {
					userID = "dev_user"
				}
				role := strings.TrimSpace(r.Header.Get("X-PawIt-Role"))
				if role == "" {
					role = "SuperAdmin"
				}
				auth = AuthContext{UserID: userID, Role: role, TenantID: tenantID}
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
	signingKey := s.jwtSigningKey()
	if signingKey == "" {
		return AuthContext{}, errors.New("missing signing key")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return AuthContext{}, errors.New("malformed token")
	}

	var header struct {
		Algorithm string `json:"alg"`
	}
	headerPayload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AuthContext{}, err
	}
	if err := json.Unmarshal(headerPayload, &header); err != nil {
		return AuthContext{}, err
	}
	if header.Algorithm != "HS256" {
		return AuthContext{}, errors.New("unsupported token algorithm")
	}

	mac := hmac.New(sha256.New, []byte(signingKey))
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

func (s *Server) signToken(auth AuthContext, expiresAt time.Time) (string, error) {
	signingKey := s.jwtSigningKey()
	if signingKey == "" {
		return "", errors.New("missing signing key")
	}

	header, err := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]any{
		"sub":       auth.UserID,
		"role":      auth.Role,
		"tenant_id": auth.TenantID,
		"exp":       expiresAt.Unix(),
		"iat":       time.Now().Unix(),
	})
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := encodedHeader + "." + encodedPayload

	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(unsigned))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + signature, nil
}

func (s *Server) jwtSigningKey() string {
	if s.cfg.JWTSigningKey != "" {
		return s.cfg.JWTSigningKey
	}
	if s.cfg.AllowDevAuth && !s.cfg.IsProduction() {
		return "pawit-local-development-signing-key"
	}
	return ""
}

func publicAuthPath(path string) bool {
	switch path {
	case "/api/v1/auth/login", "/api/v1/auth/logout":
		return true
	default:
		return false
	}
}

func (s *Server) authCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     "pawit_access",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.IsProduction(),
	}
}

func (s *Server) expiredAuthCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "pawit_access",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.cfg.IsProduction(),
	}
}
