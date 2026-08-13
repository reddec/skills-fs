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
func newServer(t *testing.T, mountAuth string) (*httptest.Server, *dbo.Queries) {
	t.Helper()
	q, err := dbo.NewFromFile(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	handler, err := server.New(context.Background(), server.Config{DB: q, MountAuth: server.MountAuth(mountAuth)})
	require.NoError(t, err)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts, q
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
	ts, _ := newServer(t, "none")
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
	ts, _ := newServer(t, "none")
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
	ts, _ := newServer(t, "none")
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
	ts, _ := newServer(t, "none")
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
	ts, _ := newServer(t, "none")
	createSkill(t, ts, `{"name":"dup","description":"d","body":"x"}`)
	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/skills", `{"name":"dup","description":"d","body":"x"}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestCreateInvalidName(t *testing.T) {
	ts, _ := newServer(t, "none")
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
	ts, _ := newServer(t, "none")
	resp, err := ts.Client().Get(ts.URL + "/api/v1/skills/9999")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestTokenMountAuth(t *testing.T) {
	ts, _ := newServer(t, "token")

	// Tokens are created through the (unauthenticated, in test) admin API.
	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/tokens", `{"label":"laptop"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created api.TokenCreated
	require.NoError(t, decodeJSON(resp.Body, &created))
	require.NotEmpty(t, created.Token)

	// No credentials -> 401.
	noAuth, err := ts.Client().Get(ts.URL + "/fs/")
	require.NoError(t, err)
	noAuth.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, noAuth.StatusCode)

	// Valid token as basic-auth password (username ignored) -> 200.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/fs/", nil)
	req.SetBasicAuth("ignored", created.Token)
	ok, err := ts.Client().Do(req)
	require.NoError(t, err)
	ok.Body.Close()
	assert.Equal(t, http.StatusOK, ok.StatusCode)

	// Wrong token -> 401.
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/fs/", nil)
	req.SetBasicAuth("ignored", "sk_wrong")
	bad, err := ts.Client().Do(req)
	require.NoError(t, err)
	bad.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, bad.StatusCode)
}

func TestFSListingAndRange(t *testing.T) {
	ts, _ := newServer(t, "none")
	createSkill(t, ts, `{"name":"code-review","description":"Review Go code.","body":"# Body line one\n# Body line two"}`)

	// Root listing: one directory per skill.
	root, err := ts.Client().Get(ts.URL + "/fs/")
	require.NoError(t, err)
	body, _ := io.ReadAll(root.Body)
	root.Body.Close()
	assert.Equal(t, http.StatusOK, root.StatusCode)
	assert.Contains(t, string(body), `<a href="code-review/">code-review/</a>`)
	assert.Equal(t, "no-store", root.Header.Get("Cache-Control"))

	// Skill directory lists SKILL.md.
	dir, err := ts.Client().Get(ts.URL + "/fs/code-review/")
	require.NoError(t, err)
	dirBody, _ := io.ReadAll(dir.Body)
	dir.Body.Close()
	assert.Equal(t, http.StatusOK, dir.StatusCode)
	assert.Contains(t, string(dirBody), `<a href="SKILL.md">SKILL.md</a>`)

	// SKILL.md has valid frontmatter + body and advertises Range support.
	file, err := ts.Client().Get(ts.URL + "/fs/code-review/SKILL.md")
	require.NoError(t, err)
	fileBody, _ := io.ReadAll(file.Body)
	file.Body.Close()
	assert.Equal(t, http.StatusOK, file.StatusCode)
	assert.Equal(t, "bytes", file.Header.Get("Accept-Ranges"))
	assert.True(t, strings.HasPrefix(string(fileBody), "---\n"), string(fileBody))
	assert.Contains(t, string(fileBody), "name: code-review")
	assert.Contains(t, string(fileBody), "description: Review Go code.")

	// Range request -> 206 partial content.
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/fs/code-review/SKILL.md", nil)
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
