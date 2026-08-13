# skills-fs

Single-user, self-hosted service that manages **Agent Skills** (one `SKILL.md` per skill
directory, per [agentskills.io](https://agentskills.io)) and serves them as a read-only HTTP
filesystem mountable with [httpdirfs](https://github.com/fangfufu/httpdirfs). One binary; the
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

Routes: `/` admin SPA + `/api/v1` admin API; `/fs/` read-only filesystem.

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

## Mounting with httpdirfs

Open the admin UI, create a skill, then (if `--mount-auth token`) issue a token — the token
dialog shows a ready-to-copy command. Without auth:

```bash
httpdirfs https://skills.example/fs/ /mnt/skills
```

> **Privacy:** mount **without** `--cache` so skill content stays in RAM and is not written to
> the local disk. `/fs` responses set `Cache-Control: no-store`.

## Layout

```
openapi.yaml        # API contract → ogen generates internal/api
internal/dbo        # sqlc + SQLite (CGO-free, modernc.org/sqlite)
internal/server     # handlers: admin API, auth, read-only /fs
internal/web        # embedded SPA
frontend            # React + Vite + Tailwind + shadcn/ui
```
