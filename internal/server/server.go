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
	"github.com/reddec/skills-fs/internal/web"
)

// ErrInvalidAuthMode signals an unrecognized admin or mount authentication mode at startup.
var ErrInvalidAuthMode = errors.New("invalid auth mode")

//nolint:gochecknoglobals // package-level logger is the project convention
var logger = slog.Default().With("controller", "server")

// AdminAuth selects how the admin panel and API are protected.
type AdminAuth string

const (
	AdminNone  AdminAuth = "none"
	AdminBasic AdminAuth = "basic"
	AdminOIDC  AdminAuth = "oidc"
)

// MountAuth selects how the read-only /fs mount is protected.
type MountAuth string

const (
	MountNone  MountAuth = "none"
	MountToken MountAuth = "token"
)

// Config carries the dependencies and authentication settings assembled by main.
type Config struct {
	DB *dbo.Queries

	AdminAuth     AdminAuth
	AdminUser     string
	AdminPassword string

	MountAuth MountAuth

	OIDC OIDCConfig
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
	q *dbo.Queries
}

// New builds the root handler: /api/v1 (admin auth), / (admin auth, SPA), /fs (mount auth).
func New(ctx context.Context, cfg Config) (http.Handler, error) {
	if err := validateAuthModes(cfg); err != nil {
		return nil, err
	}

	svc := &Server{q: cfg.DB}
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
	// (206) partial responses that httpdirfs relies on.
	adminHandler = gziphandler.GzipHandler(adminHandler)

	mountHandler := wrapMount(http.StripPrefix("/fs", newHTTPFS(cfg.DB)), cfg)

	root := http.NewServeMux()
	root.Handle("/fs/", mountHandler)
	root.Handle("/", adminHandler)
	return root, nil
}

func validateAuthModes(cfg Config) error {
	switch cfg.AdminAuth {
	case AdminNone, "", AdminBasic, AdminOIDC:
	default:
		return fmt.Errorf("%w: admin auth %q", ErrInvalidAuthMode, cfg.AdminAuth)
	}
	switch cfg.MountAuth {
	case MountNone, "", MountToken:
	default:
		return fmt.Errorf("%w: mount auth %q", ErrInvalidAuthMode, cfg.MountAuth)
	}
	return nil
}
