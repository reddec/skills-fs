package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/reddec/skills-fs/internal/api"
	"github.com/reddec/skills-fs/internal/dbo"
)

// skillNameRe enforces the Agent Skills spec for the directory/frontmatter name: lowercase
// letters and digits, single hyphens between segments, 1-64 characters.
var skillNameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const msgSkillNotFound = "skill not found"

// CreateSkill implements POST /skills.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) CreateSkill(ctx context.Context, req *api.SkillWrite) (api.CreateSkillRes, error) {
	if msg := validateSkill(req); msg != "" {
		return &api.CreateSkillBadRequest{Message: msg}, nil
	}
	meta, err := encodeMetadata(req.Metadata)
	if err != nil {
		logger.Error("encode metadata", "method", "CreateSkill", "error", err)
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	id, err := s.q.CreateSkill(ctx, dbo.CreateSkillParams{
		Name:          req.Name,
		Description:   req.Description,
		Body:          req.Body,
		License:       req.License.Or(""),
		Compatibility: req.Compatibility.Or(""),
		AllowedTools:  req.AllowedTools.Or(""),
		Metadata:      meta,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return &api.CreateSkillConflict{Message: "a skill with this name already exists"}, nil
		}
		logger.Error("create skill", "method", "CreateSkill", "error", err)
		return nil, fmt.Errorf("create skill: %w", err)
	}
	created, err := s.q.GetSkill(ctx, id)
	if err != nil {
		logger.Error("fetch created skill", "method", "CreateSkill", "error", err)
		return nil, fmt.Errorf("fetch created skill: %w", err)
	}
	out := skillToAPI(created)
	return &out, nil
}

// ListSkills implements GET /skills.
func (s *Server) ListSkills(ctx context.Context) ([]api.SkillSummary, error) {
	rows, err := s.q.ListSkills(ctx)
	if err != nil {
		logger.Error("list skills", "method", "ListSkills", "error", err)
		return nil, fmt.Errorf("list skills: %w", err)
	}
	out := make([]api.SkillSummary, len(rows))
	for i, r := range rows {
		out[i] = api.SkillSummary{
			ID:            r.ID,
			Name:          r.Name,
			Description:   r.Description,
			License:       r.License,
			Compatibility: r.Compatibility,
			AllowedTools:  r.AllowedTools,
			Metadata:      decodeMetadata(r.Metadata),
			CreatedAt:     r.CreatedAt,
			UpdatedAt:     r.UpdatedAt,
		}
	}
	return out, nil
}

// GetSkill implements GET /skills/{id}.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) GetSkill(ctx context.Context, params api.GetSkillParams) (api.GetSkillRes, error) {
	sk, err := s.q.GetSkill(ctx, params.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &api.Error{Message: msgSkillNotFound}, nil
		}
		logger.Error("get skill", "method", "GetSkill", "error", err)
		return nil, fmt.Errorf("get skill: %w", err)
	}
	out := skillToAPI(sk)
	return &out, nil
}

// UpdateSkill implements PUT /skills/{id}.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) UpdateSkill(ctx context.Context, req *api.SkillWrite, params api.UpdateSkillParams) (api.UpdateSkillRes, error) {
	if msg := validateSkill(req); msg != "" {
		return &api.UpdateSkillBadRequest{Message: msg}, nil
	}
	if _, err := s.q.GetSkill(ctx, params.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &api.UpdateSkillNotFound{Message: msgSkillNotFound}, nil
		}
		logger.Error("check skill before update", "method", "UpdateSkill", "error", err)
		return nil, fmt.Errorf("get skill: %w", err)
	}
	meta, err := encodeMetadata(req.Metadata)
	if err != nil {
		logger.Error("encode metadata", "method", "UpdateSkill", "error", err)
		return nil, fmt.Errorf("encode metadata: %w", err)
	}
	if err := s.q.UpdateSkill(ctx, dbo.UpdateSkillParams{
		Name:          req.Name,
		Description:   req.Description,
		Body:          req.Body,
		License:       req.License.Or(""),
		Compatibility: req.Compatibility.Or(""),
		AllowedTools:  req.AllowedTools.Or(""),
		Metadata:      meta,
		ID:            params.ID,
	}); err != nil {
		if isUniqueViolation(err) {
			return &api.UpdateSkillConflict{Message: "a skill with this name already exists"}, nil
		}
		logger.Error("update skill", "method", "UpdateSkill", "error", err)
		return nil, fmt.Errorf("update skill: %w", err)
	}
	updated, err := s.q.GetSkill(ctx, params.ID)
	if err != nil {
		logger.Error("fetch updated skill", "method", "UpdateSkill", "error", err)
		return nil, fmt.Errorf("fetch updated skill: %w", err)
	}
	out := skillToAPI(updated)
	return &out, nil
}

// DeleteSkill implements DELETE /skills/{id}.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) DeleteSkill(ctx context.Context, params api.DeleteSkillParams) (api.DeleteSkillRes, error) {
	n, err := s.q.DeleteSkill(ctx, params.ID)
	if err != nil {
		logger.Error("delete skill", "method", "DeleteSkill", "error", err)
		return nil, fmt.Errorf("delete skill: %w", err)
	}
	if n == 0 {
		return &api.Error{Message: msgSkillNotFound}, nil
	}
	return &api.DeleteSkillNoContent{}, nil
}

func validateSkill(req *api.SkillWrite) string {
	const (
		maxNameLen        = 64
		maxDescriptionLen = 1024
		maxCompatLen      = 500
	)
	name := req.Name
	if !skillNameRe.MatchString(name) || len(name) > maxNameLen {
		return "name must be 1-64 lowercase letters, digits, or single hyphens; no leading/trailing/consecutive hyphens"
	}
	if req.Description == "" || len(req.Description) > maxDescriptionLen {
		return "description must be 1-1024 characters"
	}
	if compat := req.Compatibility.Or(""); len(compat) > maxCompatLen {
		return "compatibility must be at most 500 characters"
	}
	return ""
}

func skillToAPI(s dbo.Skill) api.Skill {
	return api.Skill{
		ID:            s.ID,
		Name:          s.Name,
		Description:   s.Description,
		License:       s.License,
		Compatibility: s.Compatibility,
		AllowedTools:  s.AllowedTools,
		Metadata:      decodeMetadata(s.Metadata),
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
		Body:          s.Body,
	}
}

func encodeMetadata(m api.OptMetadata) (string, error) {
	val := m.Or(api.Metadata{})
	if len(val) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(val)
	if err != nil {
		return "", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(b), nil
}

func decodeMetadata(s string) api.Metadata {
	m := api.Metadata{}
	if s == "" {
		return m
	}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return api.Metadata{}
	}
	return m
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
