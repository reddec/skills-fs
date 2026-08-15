package server_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/reddec/skills-fs/internal/api"
	"github.com/reddec/skills-fs/internal/dbo"
	"github.com/reddec/skills-fs/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLLM is a minimal OpenAI-compatible /chat/completions server driving scripted
// agent conversations. Each script entry is one request in order: a JSON string is sent
// back as a submit_skill tool call, "text:<msg>" as a plain assistant reply, "error" as
// an HTTP 500.
type fakeLLM struct {
	ts     *httptest.Server
	mu     sync.Mutex
	script []string
	step   int
}

func newFakeLLM(t *testing.T, script []string) *fakeLLM {
	t.Helper()
	f := &fakeLLM{script: script}
	f.ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		f.mu.Lock()
		step := f.step
		f.step++
		f.mu.Unlock()
		if step >= len(f.script) {
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		entry := f.script[step]
		switch {
		case entry == "error":
			http.Error(w, "model exploded", http.StatusInternalServerError)
		case strings.HasPrefix(entry, "text:"):
			fmt.Fprint(w, assistantTextResponse(strings.TrimPrefix(entry, "text:")))
		default:
			fmt.Fprint(w, toolCallResponse(entry))
		}
	}))
	t.Cleanup(f.ts.Close)
	return f
}

// toolCallResponse emits an assistant message with one submit_skill tool call; the
// arguments JSON is embedded via %q so it arrives verbatim.
func toolCallResponse(args string) string {
	return fmt.Sprintf(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"test",`+
		`"choices":[{"index":0,"message":{"role":"assistant","content":null,`+
		`"tool_calls":[{"id":"call_1","type":"function","function":{"name":"submit_skill","arguments":%q}}]},`+
		`"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`, args)
}

func assistantTextResponse(text string) string {
	return fmt.Sprintf(`{"id":"chatcmpl-2","object":"chat.completion","created":1,"model":"test",`+
		`"choices":[{"index":0,"message":{"role":"assistant","content":%q},"finish_reason":"stop"}],`+
		`"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`, text)
}

// newServerWithLLM builds the full-wiring handler with skill generation enabled against
// the given OpenAI-compatible base URL.
func newServerWithLLM(t *testing.T, llmBase string) *httptest.Server {
	t.Helper()
	q, err := dbo.NewFromFile(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	handler, err := server.New(context.Background(), server.Config{
		DB: q,
		LLM: server.LLMConfig{
			BaseURL: llmBase,
			APIKey:  "test-key",
			Model:   "test-model",
		},
	})
	require.NoError(t, err)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	return ts
}

// waitForGeneration polls the job until it leaves "running" or the deadline passes.
func waitForGeneration(t *testing.T, client *http.Client, url, id string) api.Generation {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		resp, err := client.Get(url + "/api/v1/generate/" + id)
		require.NoError(t, err)
		var job api.Generation
		require.NoError(t, decodeJSON(resp.Body, &job))
		resp.Body.Close()
		if job.Status != api.GenerationStatusRunning {
			return job
		}
		if time.Now().After(deadline) {
			t.Fatal("generation job did not finish in time")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestGenerateSkillEndToEnd(t *testing.T) {
	args := `{"name":"Generated Skill","description":"Automates conventional commit messages. Use when writing commit messages.","content":"# Conventional commits\n\n1. Parse the diff.\n2. Suggest a message in conventional format."}`
	fake := newFakeLLM(t, []string{args, "text:Done"})
	ts := newServerWithLLM(t, fake.ts.URL+"/v1")

	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/generate", `{"idea":"a skill for conventional commits"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	var created api.GenerationCreated
	require.NoError(t, decodeJSON(resp.Body, &created))
	assert.True(t, strings.HasPrefix(created.ID, "gen_"))

	job := waitForGeneration(t, ts.Client(), ts.URL, created.ID)
	assert.Equal(t, api.GenerationStatusDone, job.Status)
	require.True(t, job.SkillId.Set)
	require.True(t, job.SkillName.Set)
	// "Generated Skill" is normalized to a spec-valid slug.
	assert.Equal(t, "generated-skill", job.SkillName.Value)

	// The result is a real skill through the normal API.
	resp, err := ts.Client().Get(ts.URL + "/api/v1/skills/" + strconv.FormatInt(job.SkillId.Value, 10))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var sk api.Skill
	require.NoError(t, decodeJSON(resp.Body, &sk))
	assert.Equal(t, "generated-skill", sk.Name)
	assert.Contains(t, sk.Body, "Conventional commits")

	// And it is served on the mount.
	token := createToken(t, ts, "e2e")
	resp, err = ts.Client().Do(mountRequest(t, ts, token, "/fs/generated-skill/SKILL.md"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "# Conventional commits")
}

// mountRequest builds a token-authenticated GET against the /fs mount.
func mountRequest(t *testing.T, ts *httptest.Server, token, path string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	require.NoError(t, err)
	req.SetBasicAuth("ignored", token)
	return req
}

func TestGenerateDisabled(t *testing.T) {
	ts, _ := newServer(t)

	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/generate", `{"idea":"whatever"}`)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var e api.Error
	require.NoError(t, decodeJSON(resp.Body, &e))
	assert.Contains(t, e.Message, "SKILLSFS_LLM_API_KEY")

	// The config endpoint reflects the disabled state.
	resp, err := ts.Client().Get(ts.URL + "/api/v1/config")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var cfg api.ServerConfig
	require.NoError(t, decodeJSON(resp.Body, &cfg))
	assert.False(t, cfg.Llm.Enabled)
}

func TestGetGenerationNotFound(t *testing.T) {
	ts, _ := newServer(t)
	resp, err := ts.Client().Get(ts.URL + "/api/v1/generate/does-not-exist")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGenerateAgentFailure(t *testing.T) {
	fake := newFakeLLM(t, []string{"error"})
	ts := newServerWithLLM(t, fake.ts.URL+"/v1")

	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/generate", `{"idea":"anything"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	var created api.GenerationCreated
	require.NoError(t, decodeJSON(resp.Body, &created))

	job := waitForGeneration(t, ts.Client(), ts.URL, created.ID)
	assert.Equal(t, api.GenerationStatusError, job.Status)
	require.True(t, job.Error.Set)
	assert.Contains(t, job.Error.Value, "agent run")
}

func TestGenerateAgentNoSubmission(t *testing.T) {
	fake := newFakeLLM(t, []string{"text:I would love to help, let me think about it"})
	ts := newServerWithLLM(t, fake.ts.URL+"/v1")

	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/generate", `{"idea":"anything"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	var created api.GenerationCreated
	require.NoError(t, decodeJSON(resp.Body, &created))

	job := waitForGeneration(t, ts.Client(), ts.URL, created.ID)
	assert.Equal(t, api.GenerationStatusError, job.Status)
	require.True(t, job.Error.Set)
	assert.Contains(t, job.Error.Value, "without submitting")
}

func TestGenerateInvalidSubmission(t *testing.T) {
	args := `{"name":"ok-name","description":"","content":"body"}`
	fake := newFakeLLM(t, []string{args, "text:ok"})
	ts := newServerWithLLM(t, fake.ts.URL+"/v1")

	resp := postJSON(t, ts.Client(), ts.URL+"/api/v1/generate", `{"idea":"anything"}`)
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)
	var created api.GenerationCreated
	require.NoError(t, decodeJSON(resp.Body, &created))

	job := waitForGeneration(t, ts.Client(), ts.URL, created.ID)
	assert.Equal(t, api.GenerationStatusError, job.Status)
	require.True(t, job.Error.Set)
	assert.Contains(t, job.Error.Value, "description must be 1-1024 characters")
}
