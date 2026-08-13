package server

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/reddec/skills-fs/internal/dbo"
	"gopkg.in/yaml.v3"
)

// httpfsLogger scopes log lines to the read-only filesystem handler.
var httpfsLogger = slog.Default().With("controller", "httpfs") //nolint:gochecknoglobals // package-level logger is convention

const yamlIndent = 2

// newHTTPFS builds the httpdirfs-compatible read-only filesystem rooted at the skills list.
func newHTTPFS(q *dbo.Queries) http.Handler {
	h := &httpfsHandler{q: q}
	return http.HandlerFunc(h.ServeHTTP)
}

type httpfsHandler struct {
	q *dbo.Queries
}

func (h *httpfsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	p := r.URL.Path
	if p == "" {
		p = "/"
	}
	if p == "/" {
		h.listRoot(w, r)
		return
	}

	// Split "/{name}" from the optional remainder ("/{name}/..." or "/{name}/SKILL.md").
	name, tail, _ := strings.Cut(strings.TrimPrefix(p, "/"), "/")

	switch {
	case tail == "" && strings.HasSuffix(p, "/"):
		h.listSkill(w, r, name)
	case tail == "":
		// "/{name}" -> redirect to the directory form so relative links resolve canonically.
		if !skillNameRe.MatchString(name) {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "./"+name+"/", http.StatusMovedPermanently) //nolint:gosec // G710: relative redirect of a validated single path segment
	case tail == "SKILL.md":
		h.serveSkill(w, r, name)
	default:
		http.NotFound(w, r)
	}
}

func (h *httpfsHandler) listRoot(w http.ResponseWriter, r *http.Request) {
	rows, err := h.q.ListSkills(r.Context())
	if err != nil {
		httpfsLogger.Error("list skills", "method", "listRoot", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, indexHead, "/", "/")
	for _, s := range rows {
		n := html.EscapeString(s.Name)
		fmt.Fprintf(&b, "<a href=\"%s/\">%s/</a>\n", n, n)
	}
	b.WriteString(indexTail)
	writeNoStore(w, "text/html; charset=utf-8")
	_, _ = io.WriteString(w, b.String())
}

func (h *httpfsHandler) listSkill(w http.ResponseWriter, r *http.Request, name string) {
	if _, err := h.q.GetSkillByName(r.Context(), name); err != nil {
		http.NotFound(w, r)
		return
	}
	title := html.EscapeString("/" + name + "/")
	var b strings.Builder
	fmt.Fprintf(&b, indexHead, title, title)
	b.WriteString("<a href=\"../\">../</a>\n")
	b.WriteString("<a href=\"SKILL.md\">SKILL.md</a>\n")
	b.WriteString(indexTail)
	writeNoStore(w, "text/html; charset=utf-8")
	_, _ = io.WriteString(w, b.String())
}

func (h *httpfsHandler) serveSkill(w http.ResponseWriter, r *http.Request, name string) {
	sk, err := h.q.GetSkillByName(r.Context(), name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		httpfsLogger.Error("get skill", "method", "serveSkill", "name", name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	rendered := renderSkillFile(sk)
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	http.ServeContent(w, r, "SKILL.md", sk.UpdatedAt, strings.NewReader(rendered))
}

// renderSkillFile produces a SKILL.md with YAML frontmatter (required name+description, plus
// optional fields only when set) followed by the markdown body.
func renderSkillFile(s dbo.Skill) string {
	frontmatter := map[string]any{
		"name":        s.Name,
		"description": s.Description,
	}
	if s.License != "" {
		frontmatter["license"] = s.License
	}
	if s.Compatibility != "" {
		frontmatter["compatibility"] = s.Compatibility
	}
	if s.AllowedTools != "" {
		frontmatter["allowed-tools"] = s.AllowedTools
	}
	if m := decodeMetadata(s.Metadata); len(m) > 0 {
		frontmatter["metadata"] = m
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(yamlIndent)
	_ = enc.Encode(frontmatter)
	_ = enc.Close()
	buf.WriteString("---\n")
	buf.WriteString(s.Body)
	return buf.String()
}

func writeNoStore(w http.ResponseWriter, contentType string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", contentType)
}

const (
	indexHead = `<!DOCTYPE html>
<html>
<head><title>Index of %s</title></head>
<body>
<h1>Index of %s</h1>
<hr>
<pre>
`
	indexTail = `</pre>
<hr>
</body>
</html>
`
)
