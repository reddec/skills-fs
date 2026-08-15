// Package server wires the admin API (ogen-generated), the read-only HTTPFS, and the SPA into
// a single handler, applying admin and mount authentication per route tree.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/NYTimes/gziphandler"
	"github.com/reddec/skills-fs/internal/api"
	"github.com/reddec/skills-fs/internal/dbo"
	"github.com/reddec/skills-fs/internal/generate"
	"github.com/reddec/skills-fs/internal/web"
)

// ErrInvalidAuthMode signals an unrecognized or incomplete authentication configuration
// at startup.
var ErrInvalidAuthMode = errors.New("invalid auth mode")

//nolint:gochecknoglobals // package-level logger is the project convention
var logger = slog.Default().With("controller", "server")

// AdminAuth selects how the admin panel and API are protected. The /fs mount is always
// token-protected and needs no selector.
type AdminAuth string

// AdminAuth values: none (local-only use), basic, oidc. See validateAdminAuth.
const (
	AdminNone  AdminAuth = "none"
	AdminBasic AdminAuth = "basic"
	AdminOIDC  AdminAuth = "oidc"
)

type Config struct {
	DB *dbo.Queries

	AdminAuth     AdminAuth
	AdminUser     string
	AdminPassword string

	OIDC OIDCConfig

	LLM LLMConfig
}

// LLMConfig configures the optional agent-based skill generation feature. It is enabled
// when APIKey is set.
type LLMConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// OIDCConfig configures OpenID Connect when AdminAuth is AdminOIDC.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	ServerURL    string
	TrustProxy   bool
}

// Server implements the ogen-generated api.Handler against the database.
type Server struct {
	q   *dbo.Queries
	gen *generate.Generator
}

// New builds the root handler: /api/v1 (admin auth), / (admin auth, SPA), /fs (token auth).
func New(ctx context.Context, cfg Config) (http.Handler, error) {
	if err := validateAdminAuth(cfg); err != nil {
		return nil, err
	}

	svc := &Server{
		q: cfg.DB,
		gen: generate.New(generate.Config{
			BaseURL: cfg.LLM.BaseURL,
			APIKey:  cfg.LLM.APIKey,
			Model:   cfg.LLM.Model,
		}, cfg.DB),
	}
	apiServer, err := api.NewServer(svc)
	if err != nil {
		return nil, fmt.Errorf("create api server: %w", err)
	}

	adminMux := http.NewServeMux()
	adminMux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiServer))
	adminMux.Handle("/", web.Handler())

	adminHandler, err := wrapAdmin(ctx, adminMux, cfg)
	if err != nil {
		return nil, err
	}
	// gzip covers only the admin routes: compressing /fs/ responses would corrupt the Range
	// (206) partial responses that HTTP clients like rclone rely on.
	adminHandler = gziphandler.GzipHandler(adminHandler)

	mountHandler := tokenAuth(cfg.DB)(http.StripPrefix("/fs", newHTTPFS(cfg.DB)))

	root := http.NewServeMux()
	root.Handle("/fs/", mountHandler)
	root.Handle("/", adminHandler)
	return root, nil
}

func validateAdminAuth(cfg Config) error {
	switch cfg.AdminAuth {
	case AdminNone, "":
	case AdminBasic:
		if cfg.AdminUser == "" || cfg.AdminPassword == "" {
			return fmt.Errorf("%w: basic auth requires --admin-user and --admin-password", ErrInvalidAuthMode)
		}
	case AdminOIDC:
	default:
		return fmt.Errorf("%w: admin auth %q", ErrInvalidAuthMode, cfg.AdminAuth)
	}
	return nil
}
