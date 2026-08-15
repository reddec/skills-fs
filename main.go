// Command skills-fs is a single-user, self-hosted service that manages Agent Skills and
// exposes them as a read-only HTTP filesystem for mounting with rclone.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/reddec/skills-fs/internal/dbo"
	"github.com/reddec/skills-fs/internal/server"
)

// Config binds CLI flags and SKILLSFS_* environment variables (via kong.DefaultEnvars).
type Config struct {
	Bind  string `name:"bind" help:"Bind address." default:":8080"`
	Debug bool   `name:"debug" help:"Enable debug logging."`
	DB    string `name:"db" help:"SQLite database path (\"file::memory:\" for ephemeral)." default:"skills.db"`

	AdminAuth     string `name:"admin-auth" help:"Admin auth: none|basic|oidc." default:"none"`
	AdminUser     string `name:"admin-user" help:"Admin username (basic auth)." default:"admin"`
	AdminPassword string `name:"admin-password" help:"Admin password (basic auth)."`

	LLM struct {
		BaseURL string `name:"base-url" help:"OpenAI-compatible API base URL for skill generation." default:"https://api.deepseek.com/v1"`
		APIKey  string `name:"api-key" help:"API key for skill generation. When set, the Generate feature is enabled."`
		Model   string `name:"model" help:"Model used for skill generation." default:"deepseek-v4-flash"`
	} `embed:"" prefix:"llm."`

	OIDC struct {
		Issuer       string `name:"issuer" help:"OIDC issuer URL."`
		ClientID     string `name:"client-id" help:"OIDC client ID."`
		ClientSecret string `name:"client-secret" help:"OIDC client secret."`
		ServerURL    string `name:"server-url" help:"Public server URL for OIDC redirects."`
		TrustProxy   bool   `name:"trust-proxy" help:"Trust X-Forwarded-* headers for OIDC."`
	} `embed:"" prefix:"oidc."`

	TLS struct {
		Enabled bool   `name:"enabled" help:"Enable TLS." default:"false"`
		Cert    string `name:"cert" help:"TLS certificate file." default:"/etc/tls/tls.crt"`
		Key     string `name:"key" help:"TLS key file." default:"/etc/tls/tls.key"`
	} `embed:"" prefix:"tls."`
}

func main() {
	var cfg Config
	kong.Parse(&cfg,
		kong.Name("skills-fs"),
		kong.Description("Manage Agent Skills and serve them as a read-only HTTP filesystem."),
		kong.DefaultEnvars("SKILLSFS"),
	)
	if cfg.Debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	if err := run(cfg); err != nil {
		panic(err)
	}
}

func run(cfg Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := dbo.NewFromFile(cfg.DB)
	if err != nil {
		return err
	}
	defer db.Close()
	handler, err := server.New(ctx, server.Config{
		DB:            db,
		AdminAuth:     server.AdminAuth(cfg.AdminAuth),
		AdminUser:     cfg.AdminUser,
		AdminPassword: cfg.AdminPassword,
		OIDC: server.OIDCConfig{
			Issuer:       cfg.OIDC.Issuer,
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			ServerURL:    cfg.OIDC.ServerURL,
			TrustProxy:   cfg.OIDC.TrustProxy,
		},
		LLM: server.LLMConfig{
			BaseURL: cfg.LLM.BaseURL,
			APIKey:  cfg.LLM.APIKey,
			Model:   cfg.LLM.Model,
		},
	})
	if err != nil {
		return err
	}

	if (cfg.AdminAuth == "" || cfg.AdminAuth == "none") && !loopbackBind(cfg.Bind) {
		slog.Warn("admin panel and API are UNAUTHENTICATED and reachable from the network; set --admin-auth basic|oidc or bind to loopback")
	}

	handler = securityHeaders(handler, cfg.TLS.Enabled)
	handler = http.NewCrossOriginProtection().Handler(handler)

	httpServer := http.Server{
		Addr:              cfg.Bind,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	var wg sync.WaitGroup
	wg.Go(func() {
		<-ctx.Done()
		slog.Info("shutting down server", "cause", context.Cause(ctx))
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil { //nolint:contextcheck // parent ctx is already cancelled during shutdown
			slog.Error("shutdown failed", "error", err)
		}
	})
	wg.Go(func() {
		defer cancel()
		if cfg.TLS.Enabled {
			slog.Info("starting server with TLS", "bind", cfg.Bind)
			if err := httpServer.ListenAndServeTLS(cfg.TLS.Cert, cfg.TLS.Key); !errors.Is(err, http.ErrServerClosed) {
				slog.Error("server failed", "error", err)
			}
			return
		}
		slog.Info("starting server", "bind", cfg.Bind)
		if err := httpServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
		}
	})

	slog.Info("started")
	wg.Wait()
	return nil
}

// contentSecurityPolicy restricts the SPA to same-origin resources. Script loading is
// fully strict; style-src additionally allows 'unsafe-inline' because component libraries
// (Radix, sonner) inject <style> tags at runtime.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"connect-src 'self'; " +
	"font-src 'self'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self'; " +
	"base-uri 'self'; " +
	"object-src 'none'"

// securityHeaders adds a baseline set of OWASP response headers; HSTS is added only under TLS.
func securityHeaders(next http.Handler, tls bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-XSS-Protection", "0")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		if tls || r.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		next.ServeHTTP(w, r)
	})
}

// loopbackBind reports whether the bind address restricts serving to the local host
// (empty host means all interfaces and is therefore public).
func loopbackBind(bind string) bool {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		host = bind
	}
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)
