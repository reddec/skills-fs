package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/reddec/skills-fs/internal/api"
	"github.com/reddec/skills-fs/internal/generate"
)

const msgGenerationDisabled = "skill generation is disabled: set SKILLSFS_LLM_API_KEY to enable it"

// CreateGeneration implements POST /generate. The job runs in the background; the client
// polls GetGeneration for the result, and may close the page without interrupting it.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) CreateGeneration(_ context.Context, req *api.GenerationWrite) (api.CreateGenerationRes, error) {
	//nolint:contextcheck // generation jobs must outlive the request context (async by design)
	id, err := s.gen.Start(req.Idea)
	if err != nil {
		if errors.Is(err, generate.ErrDisabled) {
			return &api.Error{Message: msgGenerationDisabled}, nil
		}
		logger.Error("start generation", "method", "CreateGeneration", "error", err)
		return nil, fmt.Errorf("start generation: %w", err)
	}
	return &api.GenerationCreated{ID: id}, nil
}

// GetGeneration implements GET /generate/{id}.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) GetGeneration(_ context.Context, params api.GetGenerationParams) (api.GetGenerationRes, error) {
	job, ok := s.gen.Get(params.ID)
	if !ok {
		return &api.Error{Message: "generation job not found"}, nil
	}
	out := api.Generation{
		ID:        job.ID,
		Status:    api.GenerationStatus(job.Status),
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
	switch job.Status {
	case generate.StatusRunning:
		// only the base fields are set
	case generate.StatusDone:
		out.SkillId = api.OptInt64{Value: job.SkillID, Set: true}
		out.SkillName = api.OptString{Value: job.SkillName, Set: true}
	case generate.StatusError:
		out.Error = api.OptString{Value: job.Error, Set: true}
	}
	return &out, nil
}

// GetConfig implements GET /config.
func (s *Server) GetConfig(_ context.Context) (*api.ServerConfig, error) {
	return &api.ServerConfig{
		Llm: api.ServerConfigLlm{
			Enabled: s.gen.Enabled(),
			Model:   s.gen.Model(),
		},
	}, nil
}
