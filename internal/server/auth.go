package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	oidclogin "github.com/reddec/oidc-login"
	"github.com/reddec/skills-fs/internal/dbo"
)

// wrapAdmin applies the configured admin authentication to the SPA + API handler.
func wrapAdmin(ctx context.Context, h http.Handler, cfg Config) (http.Handler, error) {
	switch cfg.AdminAuth {
	case AdminNone, "":
		return h, nil
	case AdminBasic:
		return basicAuth(cfg.AdminUser, cfg.AdminPassword, "skills-fs admin")(h), nil
	case AdminOIDC:
		login, err := oidclogin.New(ctx, oidclogin.Config{
			IssuerURL:    cfg.OIDC.Issuer,
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			ServerURL:    cfg.OIDC.ServerURL,
			TrustProxy:   cfg.OIDC.TrustProxy,
		})
		if err != nil {
			return nil, fmt.Errorf("oidc login: %w", err)
		}
		return login.Secure(h), nil
	default:
		return nil, fmt.Errorf("%w: admin auth %q", ErrInvalidAuthMode, cfg.AdminAuth)
	}
}

// basicAuth rejects requests whose credentials do not match in constant time. Repeated
// failures from one IP are throttled (see failureThrottle).
func basicAuth(user, pass, realm string) func(http.Handler) http.Handler {
	throttle := newFailureThrottle()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if throttle.blocked(ip) {
				tooManyRequests(w)
				return
			}
			u, p, ok := r.BasicAuth()
			userMatch := subtle.ConstantTimeCompare([]byte(u), []byte(user))
			passMatch := subtle.ConstantTimeCompare([]byte(p), []byte(pass))
			if !ok || userMatch != credentialsMatch || passMatch != credentialsMatch {
				throttle.record(ip)
				unauthorized(w, realm)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// tokenAuth guards the read-only /fs mount. A mount token is supplied as the basic-auth
// password (username ignored). Tokens are server-generated and high-entropy, so SHA3-256
// (no salt/KDF) is sufficient.
func tokenAuth(q *dbo.Queries) func(http.Handler) http.Handler {
	throttle := newFailureThrottle()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if throttle.blocked(ip) {
				tooManyRequests(w)
				return
			}
			_, token, ok := r.BasicAuth()
			if !ok || token == "" {
				throttle.record(ip)
				unauthorized(w, "skills-fs mount")
				return
			}
			tok, err := q.GetTokenByHash(r.Context(), hashToken(token))
			if err != nil {
				throttle.record(ip)
				unauthorized(w, "skills-fs mount")
				return
			}
			// Best-effort last-used stamp; a failure must not block a valid mount.
			_ = q.TouchToken(r.Context(), tok.TokenHash)
			next.ServeHTTP(w, r)
		})
	}
}

const (
	authFailureWindow = time.Minute // sliding window for counting auth failures
	authFailureLimit  = 10          // failures per IP within the window before throttling
)

// failureThrottle blocks IPs that accumulate too many authentication failures within a
// sliding window, blunting online brute-force of basic and mount-token credentials.
// Successful requests are never counted, so legitimate mounts are unaffected.
type failureThrottle struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newFailureThrottle() *failureThrottle {
	return &failureThrottle{hits: make(map[string][]time.Time)}
}

// blocked reports whether ip has exceeded the failure limit in the current window.
func (t *failureThrottle) blocked(ip string) bool {
	return t.prune(ip, time.Now()) >= authFailureLimit
}

// record stores one failure for ip.
func (t *failureThrottle) record(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.hits[ip] = append(t.pruneLocked(ip, now), now)
}

// prune drops expired hits for ip and returns the live count.
func (t *failureThrottle) prune(ip string, now time.Time) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pruneLocked(ip, now))
}

func (t *failureThrottle) pruneLocked(ip string, now time.Time) []time.Time {
	live := t.hits[ip][:0]
	for _, at := range t.hits[ip] {
		if now.Sub(at) < authFailureWindow {
			live = append(live, at)
		}
	}
	if len(live) == 0 {
		delete(t.hits, ip)
	}
	return live
}

// clientIP extracts the host part of RemoteAddr (the actual TCP peer; proxy headers are
// not trusted by design).
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// credentialsMatch is the constant-time compare result indicating equality.
const credentialsMatch = 1

func unauthorized(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}

func tooManyRequests(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
}
