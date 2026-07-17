# Mugon CLI

The official command-line tool for publishing and developing games on [mugon.net](https://mugon.net).

## Installation

**Shell (Mac, Linux):**
```sh
curl -fsSL https://mugon.net/install.sh | sh
```

**PowerShell (Windows):**
```powershell
irm https://mugon.net/install.ps1 | iex
```

<span style="color:orange">IMPORTANT: For installing the staging cli version use https://stage.mugon.net instead</span>

## Commands

### `mugon init`

Initializes a new project in the current directory. Runs an interactive prompt to set a project name and pick a starter template, then generates a `mugon.toml` config file.

**Templates:**

| Template | Description |
|---|---|
| `bevy` | Rust/Bevy game engine project |
| `typescript` | TypeScript project with Vite build setup |
| `minimal` | Bare-bones project with configurable source and distribution directories |

---

### `mugon login`

Stores an API key for a project so other commands can authenticate with the mugon.net API. If run inside a project directory, the project ID is read from `mugon.toml` automatically.

```sh
mugon login
```

**Flags:**

| Flag | Description |
|---|---|
| `--project-id` | Project ID to authenticate (skips prompt) |
| `--api-key` | API key to store (skips prompt, useful for CI/CD) |

**CI/CD usage:**
```sh
mugon login --project-id my-game --api-key $MUGON_API_KEY
```

Alternatively, set the `MUGON_PROJECT_API_KEY` environment variable to pass credentials without storing them on disk.

---

### `mugon logout`

Removes stored credentials for a project. If multiple projects are stored and no project ID is provided, an interactive selection prompt is shown.

```sh
mugon logout [--project-id <id>]
```

**Flags:**

| Flag | Description |
|---|---|
| `--project-id` | Project ID to log out from (skips prompt) |

---

### `mugon publish`

Builds and publishes the project as a new version on mugon.net.

```sh
mugon publish
```

1. Runs the `build` command from `mugon.toml` (prefers the `dev` scope, falls back to `default`).
2. Collects all files from `distribution-dir`.
3. Creates a new preliminary version via the API.
4. Uploads files in batches of 10 using presigned URLs.
5. Finalizes the version; the API validates the upload and activates it.

If the upload fails partway through, the preliminary version is automatically deleted.

**Requirements:**
- A `mugon.toml` must exist in the current directory.
- Credentials must be stored (run `mugon login` first) or `MUGON_PROJECT_API_KEY` must be set.
- `distribution-dir` must contain at least one file, including an `index.js` entry point.

---

### `mugon dev`

> **Not fully implemented yet.** The core server infrastructure is in place but the dev command is still a work in progress.

Starts a local development environment that mirrors how mugon.net hosts your game:

- A **parent frame** server (the mugon.net shell UI) on an auto-selected port starting at 3000.
- A **child frame** server (your game's `distribution-dir`) on the next available port, with a strict Content Security Policy applied.
- A **WebRTC relay** server for peer-to-peer features.
- A **file watcher** that re-runs the `build` command on changes.

```sh
mugon dev
```

**Requires** a `mugon.toml` in the current directory with at least one `build` command configured.

---

### `mugon run`

Runs a named command defined in `mugon.toml`.

```sh
mugon run <command-name> [--scope <scope>]
```

**Arguments:**

| Argument | Description |
|---|---|
| `command` | The `name` of the command as defined in `mugon.toml` |

**Flags:**

| Flag | Default | Description |
|---|---|---|
| `--scope` | `default` | Matches against the `scope` field in `[[command]]` entries |

The OS field is also matched automatically against the current platform (`linux`, `darwin`, `windows`). Commands with `os = "independent"` run on all platforms.

---

## Project config — `mugon.toml`

Every project has a `mugon.toml` in its root directory. This file is created by `mugon init` and read by `mugon publish`, `mugon dev`, and `mugon run`.

```toml
id = "my-game"
version = "0.1.0"
js-sdk-version = "0.1.0"

# Optional — defaults shown
distribution-dir = "dist"
source-dir = "src"
watch-paths = ["src"]   # defaults to [source-dir] if omitted

[[command]]
name = "build"
command = "npm run build"

[[command]]
name = "build"
command = "npm run build:dev"
scope = "dev"

[[command]]
name = "build"
command = "cargo build --target wasm32-unknown-unknown"
os = "linux"
```

### Fields

| Field | Required | Default | Description |
|---|---|---|---|
| `id` | Yes | — | Unique project identifier. Must match the project ID on mugon.net. |
| `version` | Yes | — | Semver version string published when running `mugon publish`. |
| `js-sdk-version` | Yes | — | Version of the mugon JS SDK the game targets. |
| `distribution-dir` | No | `dist` | Directory containing the built game files to be uploaded. |
| `source-dir` | No | `src` | Source directory. Used as the default for `watch-paths`. |
| `watch-paths` | No | `[source-dir]` | Paths (files or directories) to watch for changes during `mugon dev`. |

### `[[command]]` entries

Commands are matched by `name`, `scope`, and `os`. Multiple `[[command]]` entries can share the same `name` — they are selected based on the active scope and current OS.

| Field | Required | Default | Description |
|---|---|---|---|
| `name` | Yes | — | Command name (e.g. `build`, `install`). |
| `command` | Yes | — | Shell command to execute. |
| `scope` | No | `default` | Groups commands by context. `mugon dev` prefers `dev` scope over `default` while `mugon publish` prefers `default` scope over `dev`; `mugon run` defaults to `default`. |
| `os` | No | `independent` | OS constraint: `independent`, `linux`, `darwin`, or `windows`. |

---

## Global config — `~/.mugon/config.toml`

The global config is stored in your home directory and managed automatically by `mugon login` and `mugon logout`. You generally do not need to edit it manually.

```toml
[credentials]
my-game = "mgn_proj_xxxxxxxxxxxx"
another-game = "mgn_proj_yyyyyyyy"

# Optional: override the API endpoint (useful for stage environemtn or local dev)
mugon-net-api-url-override = "https://backend.stage.mugon.net/api/v1"
```

### Fields

| Field | Description |
|---|---|
| `credentials` | Map of project IDs to API keys. Populated by `mugon login`, cleared by `mugon logout`. |
| `mugon-net-api-url-override` | Overrides the default API base URL (`https://backend.mugon.net/api/v1`). |

Credentials can also be supplied at runtime via the `MUGON_PROJECT_API_KEY` environment variable, which takes precedence over the stored value.

---

## Repository note

This GitHub repository is a mirror of an internal monorepo. The project is maintained and developed in that internal repository.
