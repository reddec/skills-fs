# skills-fs

Single-user, self-hosted service that manages **Agent Skills** (one `SKILL.md` per skill
directory, per [agentskills.io](https://agentskills.io)) and serves them as a read-only HTTP
filesystem mountable with [rclone](https://rclone.org/http/). One binary; the
React admin UI is embedded.

## Configuration

Flags map to `SKILLSFS_*` env vars (e.g. `--bind` → `SKILLSFS_BIND`).

| Flag / Env | Default | Description |
|---|---|---|
| `--bind` / `SKILLSFS_BIND` | `:8080` | Bind address. |
| `--db` / `SKILLSFS_DB` | `skills.db` | SQLite path (`file::memory:` for ephemeral). |
| `--debug` / `SKILLSFS_DEBUG` | off | Debug logging. |
| `--admin-auth` / `SKILLSFS_ADMIN_AUTH` | `none` | Admin auth: `none`, `basic`, `oidc`. |
| `--admin-user` / `SKILLSFS_ADMIN_USER` | `admin` | Basic-auth username. |
| `--admin-password` | _(flag only)_ | Basic-auth password (not exposed as env). |
| `--mount-auth` / `SKILLSFS_MOUNT_AUTH` | `none` | `/fs` auth: `none`, `token`. |
| `--oidc.issuer` / `SKILLSFS_OIDC_ISSUER` | | OIDC issuer URL (admin-auth=oidc). |
| `--oidc.client-id` / `SKILLSFS_OIDC_CLIENT_ID` | | OIDC client ID. |
| `--oidc.client-secret` / `SKILLSFS_OIDC_CLIENT_SECRET` | | OIDC client secret. |
| `--oidc.server-url` / `SKILLSFS_OIDC_SERVER_URL` | | Public URL for OIDC redirects. |
| `--oidc.trust-proxy` / `SKILLSFS_OIDC_TRUST_PROXY` | off | Trust `X-Forwarded-*` for OIDC. |
| `--tls.enabled` / `SKILLSFS_TLS_ENABLED` | off | Enable TLS. |
| `--tls.cert` / `SKILLSFS_TLS_CERT` | `/etc/tls/tls.crt` | TLS certificate. |
| `--tls.key` / `SKILLSFS_TLS_KEY` | `/etc/tls/tls.key` | TLS key. |
| `--llm.base-url` / `SKILLSFS_LLM_BASE_URL` | `https://api.deepseek.com/v1` | OpenAI-compatible API base URL for skill generation. |
| `--llm.api-key` / `SKILLSFS_LLM_API_KEY` | | API key; when set, the **Generate** button appears in the UI. |
| `--llm.model` / `SKILLSFS_LLM_MODEL` | `deepseek-v4-flash` | Model used for skill generation. |

Routes: `/` admin SPA + `/api/v1` admin API; `/fs/` read-only filesystem.

## Generating skills (optional)

With `SKILLSFS_LLM_API_KEY` set, the Skills page shows a **Generate** button: paste a raw
idea or draft, and an agent (via [pikoagent](https://github.com/pikorun/pikoagent)) turns it
into a complete skill — name, description, and `SKILL.md` body. The generation runs in the
background: you can close the page, and the finished skill appears in the list (with a
notification) when done. The system prompt embeds the
[Agent Skills specification](https://agentskills.io/specification.md) and the
[skill-creator best practices](https://www.skills.sh/anthropics/skills/skill-creator); the
agent has no tools besides `submit_skill`, so it never fetches external sources.

> **Privacy:** your idea text is sent to the configured LLM provider (DeepSeek by default).
> Skills are only created after you trigger generation; existing skills stay local to the DB.

## Docker

```bash
docker run -d -p 8080:8080 -v skills-fs:/data \
  -e SKILLSFS_DB=/data/skills.db \
  ghcr.io/reddec/skills-fs:latest
```

## Build from source

```bash
make web      # build the SPA into internal/web/dist (embedded at go build)
make build    # go build
./skills-fs   # or: go run .
```

`go generate ./...` regenerates the `sqlc` and `ogen` code from `internal/dbo/queries` and
`openapi.yaml`.

## Mounting with rclone

The read-only `/fs/` is consumed by [rclone](https://rclone.org/http/) (single static binary,
all platforms). The admin UI's **Mount** page generates copy-paste install + mount scripts per
OS (incl. systemd/launchd auto-start) and per agent target (`~/.agents/skills`, `~/.claude/skills`).
If `--mount-auth token`, issue a token there first — it is filled into the commands. Quick form:

```bash
rclone mount :http: ~/.agents/skills --http-url 'https://skills:<TOKEN>@skills.example/fs/' \
  --vfs-cache-mode off --read-only
```

> **Privacy:** `--vfs-cache-mode off` keeps skill content in RAM (nothing written to local disk).
> `/fs` responses also set `Cache-Control: no-store`.

## Layout

```
openapi.yaml        # API contract → ogen generates internal/api
internal/dbo        # sqlc + SQLite (CGO-free, modernc.org/sqlite)
internal/server     # handlers: admin API, auth, read-only /fs
internal/generate   # background skill generation (LLM agent, async jobs)
internal/web        # embedded SPA
frontend            # React + Vite + Tailwind + shadcn/ui
```
