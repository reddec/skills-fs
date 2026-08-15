package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/reddec/skills-fs/internal/api"
	"github.com/reddec/skills-fs/internal/dbo"
	"github.com/reddec/skills-fs/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newServer builds a full-wiring handler backed by an isolated temp-file SQLite database.
// The admin auth is none (test context); the mount is always token-protected.
func newServer(t *testing.T) (*httptest.Server, *dbo.Queries) {
	t.Helper()
	q, err := dbo.NewFromFile(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	handler, err := server.New(context.Background(), server.Config{DB: q})
	require.NoError(t, err)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, q
}

// createToken issues a mount token through the admin API and returns its plaintext.
func createToken(t *testing.T, ts *httptest.Server, label string) string {
	t.Helper()
	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/tokens", `{"label":"`+label+`"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created api.TokenCreated
	require.NoError(t, decodeJSON(resp.Body, &created))
	require.NotEmpty(t, created.Token)
	return created.Token
}

// getFS fetches a /fs path authenticated with a mount token (basic-auth password).
func getFS(t *testing.T, ts *httptest.Server, token, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	require.NoError(t, err)
	req.SetBasicAuth("ignored", token)
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func postJSON(t *testing.T, client *http.Client, url, body string) *http.Response {
	t.Helper()
	resp, err := client.Post(url, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	return resp
}

func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}

func createSkill(t *testing.T, ts *httptest.Server, body string) api.Skill {
	t.Helper()
	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/skills", body)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var sk api.Skill
	require.NoError(t, decodeJSON(resp.Body, &sk))
	return sk
}

func TestCreateAndGetSkill(t *testing.T) {
	ts, _ := newServer(t)
	created := createSkill(t, ts, `{"name":"code-review","description":"Review Go code.","body":"# Hi"}`)
	assert.Equal(t, "code-review", created.Name)
	assert.Equal(t, "# Hi", created.Body)

	resp, err := ts.Client().Get(ts.URL + "/api/v1/skills/" + strconv.FormatInt(created.ID, 10))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var got api.Skill
	require.NoError(t, decodeJSON(resp.Body, &got))
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, "# Hi", got.Body)
}

func TestListSkills(t *testing.T) {
	ts, _ := newServer(t)
	createSkill(t, ts, `{"name":"alpha","description":"d","body":"a"}`)
	createSkill(t, ts, `{"name":"beta","description":"d","body":"b"}`)

	resp, err := ts.Client().Get(ts.URL + "/api/v1/skills")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var list []api.SkillSummary
	require.NoError(t, decodeJSON(resp.Body, &list))
	assert.Len(t, list, 2)
	assert.Equal(t, "alpha", list[0].Name) // ordered by name
}

func TestUpdateSkill(t *testing.T) {
	ts, _ := newServer(t)
	sk := createSkill(t, ts, `{"name":"to-update","description":"d","body":"old"}`)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/v1/skills/"+strconv.FormatInt(sk.ID, 10), strings.NewReader(
		`{"name":"to-update","description":"changed","body":"new"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var got api.Skill
	require.NoError(t, decodeJSON(resp.Body, &got))
	assert.Equal(t, "changed", got.Description)
	assert.Equal(t, "new", got.Body)
}

func TestDeleteSkill(t *testing.T) {
	ts, _ := newServer(t)
	sk := createSkill(t, ts, `{"name":"doomed","description":"d","body":"x"}`)

	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/skills/"+strconv.FormatInt(sk.ID, 10), nil)
	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Second delete -> 404 (idempotent at the resource level).
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/skills/"+strconv.FormatInt(sk.ID, 10), nil)
	resp, err = ts.Client().Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestCreateDuplicateName(t *testing.T) {
	ts, _ := newServer(t)
	createSkill(t, ts, `{"name":"dup","description":"d","body":"x"}`)
	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/skills", `{"name":"dup","description":"d","body":"x"}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCreateInvalidName(t *testing.T) {
	ts, _ := newServer(t)
	for _, body := range []string{
		`{"name":"Bad_Case","description":"d","body":"x"}`,
		`{"name":"-leading","description":"d","body":"x"}`,
		`{"name":"double--hyphen","description":"d","body":"x"}`,
	} {
		resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/skills", body)
		resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, body)
	}
}

func TestGetMissingSkill(t *testing.T) {
	ts, _ := newServer(t)
	resp, err := ts.Client().Get(ts.URL + "/api/v1/skills/9999")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTokenMountAuth(t *testing.T) {
	ts, _ := newServer(t)
	token := createToken(t, ts, "laptop")

	// No credentials -> 401.
	noAuth, err := ts.Client().Get(ts.URL + "/fs/")
	require.NoError(t, err)
	noAuth.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, noAuth.StatusCode)

	// Valid token as basic-auth password (username ignored) -> 200.
	ok := getFS(t, ts, token, "/fs/")
	assert.Equal(t, http.StatusOK, ok.StatusCode)

	// Wrong token -> 401.
	bad := getFS(t, ts, "sk_wrong", "/fs/")
	assert.Equal(t, http.StatusUnauthorized, bad.StatusCode)
}

func TestMountAuthThrottle(t *testing.T) {
	ts, _ := newServer(t)

	// authFailureLimit wrong tokens stay 401; the next one is throttled to 429.
	for i := range 10 {
		resp := getFS(t, ts, "sk_wrong", "/fs/")
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "attempt %d", i)
	}
	throttled := getFS(t, ts, "sk_wrong", "/fs/")
	assert.Equal(t, http.StatusTooManyRequests, throttled.StatusCode)

	// Throttling applies to the IP, not the credential: even a valid token is rejected.
	token := createToken(t, ts, "late")
	blocked := getFS(t, ts, token, "/fs/")
	assert.Equal(t, http.StatusTooManyRequests, blocked.StatusCode)
}

func TestBasicAuthRequiresCredentials(t *testing.T) {
	q, err := dbo.NewFromFile(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	// Empty password must be rejected at startup (auth bypass), empty user likewise.
	_, err = server.New(context.Background(), server.Config{DB: q, AdminAuth: server.AdminBasic})
	require.ErrorIs(t, err, server.ErrInvalidAuthMode)

	_, err = server.New(context.Background(), server.Config{DB: q, AdminAuth: server.AdminBasic, AdminUser: "admin"})
	require.ErrorIs(t, err, server.ErrInvalidAuthMode)

	_, err = server.New(context.Background(), server.Config{DB: q, AdminAuth: server.AdminBasic, AdminUser: "admin", AdminPassword: "hunter2"})
	require.NoError(t, err)
}

func TestBasicAuthMount(t *testing.T) {
	q, err := dbo.NewFromFile(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	handler, err := server.New(context.Background(), server.Config{
		DB:            q,
		AdminAuth:     server.AdminBasic,
		AdminUser:     "admin",
		AdminPassword: "hunter2",
	})
	require.NoError(t, err)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	// Empty password (the old bypass) -> 401.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/skills", nil)
	req.SetBasicAuth("admin", "")
	empty, err := ts.Client().Do(req)
	require.NoError(t, err)
	empty.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, empty.StatusCode)

	// Valid credentials -> 200.
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/skills", nil)
	req.SetBasicAuth("admin", "hunter2")
	ok, err := ts.Client().Do(req)
	require.NoError(t, err)
	ok.Body.Close()
	assert.Equal(t, http.StatusOK, ok.StatusCode)
}

func TestFSListingAndRange(t *testing.T) {
	ts, _ := newServer(t)
	token := createToken(t, ts, "fs")
	createSkill(t, ts, `{"name":"code-review","description":"Review Go code.","body":"# Body line one\n# Body line two"}`)

	// Root listing: one directory per skill.
	root := getFS(t, ts, token, "/fs/")
	rootBody, _ := io.ReadAll(root.Body)
	assert.Equal(t, http.StatusOK, root.StatusCode)
	assert.Contains(t, string(rootBody), `<a href="code-review/">code-review/</a>`)
	assert.Equal(t, "no-store", root.Header.Get("Cache-Control"))

	// Skill directory lists SKILL.md.
	dir := getFS(t, ts, token, "/fs/code-review/")
	dirBody, _ := io.ReadAll(dir.Body)
	assert.Equal(t, http.StatusOK, dir.StatusCode)
	assert.Contains(t, string(dirBody), `<a href="SKILL.md">SKILL.md</a>`)

	// SKILL.md has valid frontmatter + body and advertises Range support.
	file := getFS(t, ts, token, "/fs/code-review/SKILL.md")
	fileBody, _ := io.ReadAll(file.Body)
	assert.Equal(t, http.StatusOK, file.StatusCode)
	assert.Equal(t, "bytes", file.Header.Get("Accept-Ranges"))
	assert.True(t, strings.HasPrefix(string(fileBody), "---\n"), string(fileBody))
	assert.Contains(t, string(fileBody), "name: code-review")
	assert.Contains(t, string(fileBody), "description: Review Go code.")

	// Range request -> 206 partial content.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/fs/code-review/SKILL.md", nil)
	req.SetBasicAuth("ignored", token)
	req.Header.Set("Range", "bytes=0-9")
	rng, err := ts.Client().Do(req)
	require.NoError(t, err)
	rngBody, _ := io.ReadAll(rng.Body)
	rng.Body.Close()
	assert.Equal(t, http.StatusPartialContent, rng.StatusCode)
	assert.Equal(t, "bytes", rng.Header.Get("Accept-Ranges"))
	assert.Equal(t, "bytes 0-9/"+strconv.Itoa(len(fileBody)), rng.Header.Get("Content-Range"))
	assert.Len(t, rngBody, 10)
	assert.Contains(t, rng.Header.Get("Cache-Control"), "no-store")
}
