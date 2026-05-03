# Note

Local-first notes with a SQLite search index, optional sync to a small HTTP server, and Ed25519-backed authentication. Originally sketched during an internal SCWX hackathon (September 2020); the tree now includes multi-device sync, a companion server, and agent-oriented note helpers.

## What you get

- **CLI** — Initialize an index under `~/notes`, edit `.txt` notes, list them, and full-text search.
- **Sync** — Push and pull against a configured server after `login`; conflicts can be resolved with merge (diff3), prefer-server, prefer-client, or last-write.
- **noteserver** — Separate `main` in `cmd/noteserver`: register users, store per-user note files, HTTP API under `/v1/`.
- **`note ai`** — Subcommands for `ai-*.txt` style notes (list, create, edit, delete, search, trash) for tooling and agents.

Configuration lives in **`~/.note.yaml`** (override with `--config`). Accounts use **two Ed25519 keypairs**:

- **User (identity)** — stable per person; its public key is stored on the server as `identity.pub`.
- **Device** — distinct per machine; its public key is stored on the server in `authorized_devices` (one line per device). **`um sync`** signs HTTP requests with the **device** private key.

`login` writes **`user_private_key`** / **`user_public_key`** (32-byte seeds as standard base64) to the config file — **copy this file across machines** you own. Each machine also gets **`~/notes/note_device_ed25519`** (+ `.pub`): the device signing key stays **local only** and is created or regenerated when you `login` there if the file is missing.

**Older installs** may still have **`private_key`** / **`public_key`** and `note_id_ed25519`; `login` maps those to identity + local device files.

### Example `~/.note.yaml`

```yaml
server: https://notes.example.com
user: alex
user_private_key: <base64 seed — identity; safe to copy between your devices>
user_public_key: <base64 — filled in by login>
```

Use the same base URL you pass to `login --server` (no trailing slash required; the client normalizes it).

## Requirements

- **Client** — [Go](https://go.dev/dl/) for your platform. The client uses `github.com/mattn/go-sqlite3`, so builds need **CGO** and a C toolchain unless you switch to a pure-Go driver.
- **noteserver** — Go; the provided `Dockerfile` builds a static binary with `CGO_ENABLED=0`.

## Install the client as `um`

From a clone of this repository:

```bash
./scripts/install.sh
```

That script builds the module root (`main.go` → `cmd.Execute()`) and installs the binary to **`~/bin/um`**. Ensure `~/bin` is on your `PATH`, for example:

```bash
export PATH="$HOME/bin:$PATH"
```

The Cobra CLI still identifies itself as `note` in built-in help text; invoke whatever name you installed (here, `um`).

## Quick start (local only)

```bash
um init
um edit meeting-2026-04-01    # opens your editor; saves under ~/notes
um search agenda
um ls
```

## Sync with a server

1. Run **noteserver** (from source or container) with a non-empty admin password (flag `-password` or env `NOTE_ADMIN_PASSWORD`).
2. On the machine that should sync:

```bash
um login --server https://your-host.example --user yourname --password '<admin-password>'
# Creates the account (user + first device) or, if the user already exists,
# registers this device's public key with the admin password via POST /v1/register-device.
um sync
# Later you can omit --password on login to only verify keys and reachability.
```

**Second device:** copy **`~/.note.yaml`** (or at least `server`, `user`, identity keys) — **do not** copy `~/notes/note_device_ed25519`. On the new machine run **`um login … --password '<admin-password>'`** so a **new** device key is written under `~/notes` and registered on the server; then **`um sync`** pulls notes.

Optional conflict mode:

```bash
um sync --truth merge    # default; also: server, client, lastwrite
```

## noteserver (Docker)

```bash
docker build -t noteserver .
docker run --rm -e NOTE_ADMIN_PASSWORD='...' -p 8080:8080 -v note-data:/data noteserver
```

Listen address and data directory follow the `noteserver` flags (defaults `:8080` and `/data` in the image).

### Reset server data (all users and notes)

With the same admin secret as registration:

```bash
noteserver -datadir /path/to/note_data -password '<admin-password>' -wipe-all
```

This deletes **`datadir/users`** entirely (every account, keys, and `.txt` notes), then recreates an empty `users` directory. The server process does not start; exit after wiping.

Docker example (named volume `note-data`):

```bash
docker run --rm -e NOTE_ADMIN_PASSWORD='...' -v note-data:/data noteserver -wipe-all
```

## Repository layout

| Path | Role |
|------|------|
| `main.go`, `cmd/` | CLI commands (`init`, `edit`, `search`, `ls`, `sync`, `login`, `help`, `ai`, …) |
| `sync/` | HTTP client and sync orchestration |
| `internal/auth`, `internal/paths`, `internal/merge`, `internal/syncstate`, `internal/ainotes` | Keys, layout, merge, sync metadata, AI-note helpers |
| `cmd/noteserver` | Standalone sync server |
| `analyze/` | Tokenization / indexing used by edit and sync |

## Roadmap / gaps

- **`note config`** is still a stub (`config called`).
- **`note init`** remains minimal; re-indexing all existing files from disk is not exposed as a dedicated command yet.

Patches welcome.
