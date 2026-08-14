// Package generate runs asynchronous agent-based skill generation: a user idea is turned
// into a complete Agent Skill (name, description, SKILL.md body) by an LLM agent via
// github.com/pikorun/pikoagent. Jobs run in the background, decoupled from the HTTP
// request lifecycle, so closing the browser mid-run does not cancel the work.
package generate

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pikorun/pikoagent/agent"
	"github.com/pikorun/pikoagent/tools"
	"github.com/reddec/skills-fs/internal/dbo"
)

//go:embed prompt.md
var promptFS embed.FS

//nolint:gochecknoglobals // package-level logger is the project convention
var logger = slog.Default().With("controller", "generate")

const (
	// agentTimeout bounds one generation job (agent conversation + skill insert).
	agentTimeout = 10 * time.Minute
	// maxAgentTokens caps the completion length per LLM round-trip.
	maxAgentTokens int64 = 8192
	// maxIdeaLen bounds the accepted idea text.
	maxIdeaLen = 20000
	// jobRetention caps retained job history; finished jobs beyond it are evicted.
	jobRetention = 32

	maxNameLen        = 64
	maxDescriptionLen = 1024
)

// ErrDisabled signals that generation is not configured (no API key at startup).
var ErrDisabled = errors.New("skill generation is disabled")

// ErrEmptyIdea signals an empty idea.
var ErrEmptyIdea = errors.New("idea must not be empty")

var (
	errIdeaTooLong    = errors.New("idea is too long")
	errNoSubmission   = errors.New("agent finished without submitting a skill")
	errInvalidName    = errors.New("name does not match the Agent Skills spec")
	errBadDescription = errors.New("description must be 1-1024 characters")
	errEmptyBody      = errors.New("content (SKILL.md body) must not be empty")
)

// skillNameRe enforces the Agent Skills spec name rule (same contract as internal/server).
var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// invalidNameRe matches runs of characters not allowed in a skill name (normalized to "-").
var invalidNameRe = regexp.MustCompile(`[^a-z0-9]+`)

// Config holds LLM provider settings for skill generation.
type Config struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Enabled reports whether generation is configured: an API key must be set at startup.
func (c Config) Enabled() bool {
	return c.APIKey != ""
}

// Status of a generation job.
type Status string

const (
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusError   Status = "error"
)

// Job is a snapshot of one generation run. It is safe to read any copy; only the
// generator mutates its internal entries.
type Job struct {
	ID        string
	Status    Status
	Error     string
	SkillID   int64
	SkillName string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// jobEntry is the mutable registry record behind a Job snapshot.
type jobEntry struct {
	job Job
}

// Generator runs skill-generation jobs in the background. Create with New; the zero
// value is not usable.
type Generator struct {
	cfg Config
	q   *dbo.Queries

	mu    sync.Mutex
	jobs  map[string]*jobEntry
	order []string // job ids, oldest first; only finished jobs are evicted
}

// New creates a Generator. Jobs are created with Start and observed with Get.
func New(cfg Config, q *dbo.Queries) *Generator {
	return &Generator{
		cfg:  cfg,
		q:    q,
		jobs: make(map[string]*jobEntry),
	}
}

// Enabled reports whether generation is configured.
func (g *Generator) Enabled() bool {
	return g.cfg.Enabled()
}

// Model returns the configured LLM model (for display in the UI).
func (g *Generator) Model() string {
	return g.cfg.Model
}

// Start launches a background generation job for the idea and returns its id. The job
// runs detached from the caller's context; observe progress with Get.
func (g *Generator) Start(idea string) (string, error) {
	if !g.cfg.Enabled() {
		return "", ErrDisabled
	}
	if strings.TrimSpace(idea) == "" {
		return "", ErrEmptyIdea
	}
	if len(idea) > maxIdeaLen {
		return "", fmt.Errorf("%w: %d characters (max %d)", errIdeaTooLong, len(idea), maxIdeaLen)
	}

	id, err := newJobID()
	if err != nil {
		return "", fmt.Errorf("generate job id: %w", err)
	}

	g.mu.Lock()
	g.jobs[id] = &jobEntry{job: Job{
		ID:        id,
		Status:    StatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}}
	g.order = append(g.order, id)
	g.evictLocked()
	g.mu.Unlock()

	go g.run(id, idea)
	return id, nil
}

// Get returns a snapshot of a job. The bool reports whether the id is still retained.
func (g *Generator) Get(id string) (Job, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.jobs[id]
	if !ok {
		return Job{}, false
	}
	return entry.job, true
}

// run performs one generation job: agent conversation, then skill insert. Its context
// is derived from context.Background (not the request), so the job survives the client
// closing the page; only the server's own timeout bounds it.
func (g *Generator) run(id, idea string) {
	ctx, cancel := context.WithTimeout(context.Background(), agentTimeout)
	defer cancel()

	name, description, body, err := g.generateSkill(ctx, idea)
	if err == nil {
		var skillID int64
		skillID, err = g.q.CreateSkill(ctx, dbo.CreateSkillParams{
			Name:          name,
			Description:   description,
			Body:          body,
			License:       "",
			Compatibility: "",
			AllowedTools:  "",
			Metadata:      "{}",
		})
		if err != nil {
			err = fmt.Errorf("save generated skill: %w", err)
		} else {
			g.finish(id, Job{Status: StatusDone, SkillID: skillID, SkillName: name})
			logger.Info("skill generated", "job", id, "skill", name)
			return
		}
	}
	logger.Error("skill generation failed", "job", id, "error", err)
	g.finish(id, Job{Status: StatusError, Error: err.Error()})
}

// generateSkill runs the agent conversation and returns the submitted skill.
func (g *Generator) generateSkill(ctx context.Context, idea string) (string, string, string, error) {
	system, err := promptFS.ReadFile("prompt.md")
	if err != nil {
		return "", "", "", fmt.Errorf("read system prompt: %w", err)
	}
	prompt := string(system)

	// Tell the agent which names are taken so it picks a different one (avoids 409s).
	names, err := g.q.ListSkills(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("list existing skills: %w", err)
	}
	if len(names) > 0 {
		var taken []string
		for _, s := range names {
			taken = append(taken, s.Name)
		}
		prompt += "\n\nExisting skill names — choose a different name:\n" + strings.Join(taken, "\n")
	}

	ag := agent.New(g.cfg.BaseURL, g.cfg.APIKey, g.cfg.Model,
		agent.System(prompt),
		agent.MaxTokens(maxAgentTokens),
	)

	var submitted *skillSubmission
	ag.Tools().Add(tools.Local("submit_skill", func(_ context.Context, args skillSubmission) (string, error) {
		submitted = &args
		return "Skill draft recorded. End the conversation with a brief confirmation.", nil
	}).Description("Submit the finished skill: name, description, and the complete SKILL.md Markdown body (no YAML frontmatter, no code fences)."))

	conv := ag.Prompt(idea)
	for !conv.Done() {
		if _, err := conv.Run(ctx); err != nil {
			return "", "", "", fmt.Errorf("agent run: %w", err)
		}
	}
	if submitted == nil {
		reply := clip(strings.TrimSpace(conv.Response()), maxReplyClip)
		return "", "", "", fmt.Errorf("%w (final reply: %q)", errNoSubmission, reply)
	}
	return normalizeSubmission(*submitted)
}

// skillSubmission is the structured output the agent must produce via submit_skill.
type skillSubmission struct {
	Name        string `json:"name" description:"Skill name: 1-64 lowercase letters, digits, and single hyphens."`
	Description string `json:"description" description:"1-1024 characters: what the skill does and when to use it, with trigger keywords."`
	Content     string `json:"content" description:"Complete SKILL.md Markdown body without YAML frontmatter and without code fences."`
}

// normalizeSubmission cleans and validates the agent's output against the Agent Skills
// spec. Model output is normalized where the intent is unambiguous (name casing and
// separators) and rejected where the spec is violated.
func normalizeSubmission(sub skillSubmission) (string, string, string, error) {
	name := normalizeName(sub.Name)
	description := strings.TrimSpace(sub.Description)
	body := strings.TrimSpace(sub.Content)

	switch {
	case name == "" || !skillNameRe.MatchString(name) || len(name) > maxNameLen:
		return "", "", "", fmt.Errorf("%w: %q (expected 1-64 lowercase letters, digits, or single hyphens)", errInvalidName, sub.Name)
	case description == "" || len(description) > maxDescriptionLen:
		return "", "", "", errBadDescription
	case body == "":
		return "", "", "", errEmptyBody
	}
	return name, description, body, nil
}

// normalizeName converts a model-produced title into a spec-valid slug: lowercased,
// runs of invalid characters collapsed to a single hyphen, hyphens trimmed at the edges.
func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = invalidNameRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// finish records the terminal state of a job.
func (g *Generator) finish(id string, j Job) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry, ok := g.jobs[id]
	if !ok {
		return
	}
	j.ID = id
	j.CreatedAt = entry.job.CreatedAt
	j.UpdatedAt = time.Now()
	entry.job = j
}

// evictLocked drops finished jobs beyond jobRetention, oldest first. Running jobs are
// never evicted.
func (g *Generator) evictLocked() {
	for len(g.order) > jobRetention {
		oldest := g.order[0]
		if g.jobs[oldest].job.Status == StatusRunning {
			return
		}
		delete(g.jobs, oldest)
		g.order = g.order[1:]
	}
}

// newJobID returns a random, unguessable job id.
func newJobID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "gen_" + hex.EncodeToString(b[:]), nil
}

// maxReplyClip bounds the agent's final reply included in error messages.
const maxReplyClip = 200

// clip shortens a string for inclusion in error messages.
func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}
