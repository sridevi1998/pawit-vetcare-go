package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]bucket
	limit      int
	window     time.Duration
	lastPruned time.Time
}

type bucket struct {
	count     int
	expiresAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{buckets: map[string]bucket{}, limit: limit, window: window}
}

func (r *rateLimiter) allow(key string) (bool, time.Duration) {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.lastPruned.IsZero() || now.Sub(r.lastPruned) >= r.window {
		r.pruneExpired(now)
		r.lastPruned = now
	}

	current := r.buckets[key]
	if current.expiresAt.Before(now) {
		current = bucket{expiresAt: now.Add(r.window)}
	}
	current.count++
	r.buckets[key] = current

	if current.count <= r.limit {
		return true, 0
	}
	return false, current.expiresAt.Sub(now)
}

func (r *rateLimiter) pruneExpired(now time.Time) {
	for key, current := range r.buckets {
		if current.expiresAt.Before(now) {
			delete(r.buckets, key)
		}
	}
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return s.recoverer(s.securityHeaders(s.cors(s.requestSizeLimit(s.rateLimit(s.authenticate(next))))))
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = withRequestID(r)
		w.Header().Set("X-Request-ID", requestID(r))

		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "requestId", requestID(r), "error", err)
				writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		if s.cfg.IsProduction() {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.originAllowed(origin) {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-PawIt-Tenant-ID, X-PawIt-User-ID, X-PawIt-Role, X-Request-ID")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Expose-Headers", "Retry-After, X-Request-ID")
			h.Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	for _, allowed := range s.cfg.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func (s *Server) requestSizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, s.cfg.RequestBodyLimit)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := s.clientIP(r)
		if ok, retryAfter := s.limiter.allow(key); !ok {
			w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "Too many requests. Please retry shortly.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func retryAfterSeconds(duration time.Duration) string {
	seconds := int(duration.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func (s *Server) clientIP(r *http.Request) string {
	remoteIP := remoteIP(r.RemoteAddr)
	if remoteIP != "" && s.trustsProxy(remoteIP) {
		for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
			value := strings.TrimSpace(r.Header.Get(header))
			if value == "" {
				continue
			}
			parts := strings.Split(value, ",")
			candidate := strings.TrimSpace(parts[0])
			if net.ParseIP(candidate) != nil {
				return candidate
			}
		}
	}
	if remoteIP != "" {
		return remoteIP
	}
	return r.RemoteAddr
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		if net.ParseIP(remoteAddr) != nil {
			return remoteAddr
		}
		return ""
	}
	return host
}

func (s *Server) trustsProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, value := range s.cfg.TrustedProxyCIDRs {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if trustedIP := net.ParseIP(value); trustedIP != nil && trustedIP.Equal(parsed) {
			return true
		}
		_, network, err := net.ParseCIDR(value)
		if err == nil && network.Contains(parsed) {
			return true
		}
	}
	return false
}

func withRequestID(r *http.Request) *http.Request {
	id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if id == "" {
		id = randomID()
	}
	return r.WithContext(withRequestIDContext(r.Context(), id))
}

func randomID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(bytes[:])
}
