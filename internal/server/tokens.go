package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/reddec/skills-fs/internal/api"
	"github.com/reddec/skills-fs/internal/dbo"
	"golang.org/x/crypto/sha3"
)

const (
	tokenScheme    = "sk_" // plaintext token prefix
	tokenEntropy   = 32    // random bytes; high entropy, so SHA3-256 without a salt suffices
	tokenPrefixLen = 8     // leading characters stored for identification without the secret
)

// CreateToken implements POST /tokens. The plaintext token is returned once.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) CreateToken(ctx context.Context, req *api.TokenWrite) (api.CreateTokenRes, error) {
	token, hash, prefix, err := generateToken()
	if err != nil {
		logger.Error("generate token", "method", "CreateToken", "error", err)
		return nil, fmt.Errorf("generate token: %w", err)
	}
	id, err := s.q.CreateToken(ctx, dbo.CreateTokenParams{
		Label:       req.Label,
		TokenHash:   hash,
		TokenPrefix: prefix,
	})
	if err != nil {
		logger.Error("create token", "method", "CreateToken", "error", err)
		return nil, fmt.Errorf("create token: %w", err)
	}
	created, err := s.q.GetToken(ctx, id)
	if err != nil {
		logger.Error("fetch created token", "method", "CreateToken", "error", err)
		return nil, fmt.Errorf("fetch created token: %w", err)
	}
	return &api.TokenCreated{
		ID:         created.ID,
		Label:      created.Label,
		Prefix:     created.TokenPrefix,
		LastUsedAt: nilDateTime(created.LastUsedAt),
		CreatedAt:  created.CreatedAt,
		Token:      token,
	}, nil
}

// ListTokens implements GET /tokens (no secrets).
func (s *Server) ListTokens(ctx context.Context) ([]api.Token, error) {
	rows, err := s.q.ListTokens(ctx)
	if err != nil {
		logger.Error("list tokens", "method", "ListTokens", "error", err)
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	out := make([]api.Token, len(rows))
	for i, t := range rows {
		out[i] = api.Token{
			ID:         t.ID,
			Label:      t.Label,
			Prefix:     t.TokenPrefix,
			LastUsedAt: nilDateTime(t.LastUsedAt),
			CreatedAt:  t.CreatedAt,
		}
	}
	return out, nil
}

// DeleteToken implements DELETE /tokens/{id}.
//
//nolint:ireturn // returns the ogen-generated response union required by api.Handler
func (s *Server) DeleteToken(ctx context.Context, params api.DeleteTokenParams) (api.DeleteTokenRes, error) {
	n, err := s.q.DeleteToken(ctx, params.ID)
	if err != nil {
		logger.Error("delete token", "method", "DeleteToken", "error", err)
		return nil, fmt.Errorf("delete token: %w", err)
	}
	if n == 0 {
		return &api.Error{Message: "token not found"}, nil
	}
	return &api.DeleteTokenNoContent{}, nil
}

// generateToken creates a high-entropy token together with its SHA3-256 hash (for storage
// lookup) and a short non-secret prefix (for identification in the UI).
func generateToken() (string, string, string, error) {
	var buf [tokenEntropy]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", "", "", fmt.Errorf("read random: %w", err)
	}
	token := tokenScheme + base64.RawURLEncoding.EncodeToString(buf[:])
	prefix := token
	if len(prefix) > tokenPrefixLen {
		prefix = prefix[:tokenPrefixLen]
	}
	return token, hashToken(token), prefix, nil
}

// hashToken returns the hex SHA3-256 of a token for constant-time-free indexed DB lookup.
func hashToken(token string) string {
	sum := sha3.Sum256([]byte(token)) //nolint:govet // inline: crypto primitive; inlining is the compiler's decision
	return hex.EncodeToString(sum[:])
}

func nilDateTime(t *time.Time) api.NilDateTime {
	if t == nil {
		return api.NilDateTime{Null: true}
	}
	return api.NewNilDateTime(*t)
}
