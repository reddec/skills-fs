package server

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"

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

// wrapMount applies the configured mount authentication to the /fs handler.
func wrapMount(h http.Handler, cfg Config) http.Handler {
	switch cfg.MountAuth {
	case MountNone, "":
		return h
	case MountToken:
		return tokenAuth(cfg.DB)(h)
	default:
		return h
	}
}

// basicAuth rejects requests whose credentials do not match in constant time.
func basicAuth(user, pass, realm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, p, ok := r.BasicAuth()
			userMatch := subtle.ConstantTimeCompare([]byte(u), []byte(user))
			passMatch := subtle.ConstantTimeCompare([]byte(p), []byte(pass))
			if !ok || userMatch != credentialsMatch || passMatch != credentialsMatch {
				unauthorized(w, realm)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// tokenAuth validates a mount token supplied as the basic-auth password (username ignored).
// Tokens are server-generated and high-entropy, so SHA3-256 (no salt/KDF) is sufficient.
func tokenAuth(q *dbo.Queries) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, token, ok := r.BasicAuth()
			if !ok || token == "" {
				unauthorized(w, "skills-fs mount")
				return
			}
			tok, err := q.GetTokenByHash(r.Context(), hashToken(token))
			if err != nil {
				unauthorized(w, "skills-fs mount")
				return
			}
			// Best-effort last-used stamp; a failure must not block a valid mount.
			_ = q.TouchToken(r.Context(), tok.TokenHash)
			next.ServeHTTP(w, r)
		})
	}
}

// credentialsMatch is the constant-time compare result indicating equality.
const credentialsMatch = 1

func unauthorized(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="`+realm+`", charset="UTF-8"`)
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}
