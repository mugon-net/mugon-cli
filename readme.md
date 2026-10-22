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

1. Runs the `build` command from `mugon.toml` (prefers the `default` scope, falls back to `dev`).
2. Collects all files from `distribution-dir`.
3. Creates a new preliminary version via the API.
4. Uploads files in batches of 10 using presigned URLs.
5. Finalizes the version; the API validates the upload and activates it.

Before uploading, the CLI also checks the build output: an `@mugon/sdk` has to be bundled, its
major version has to match `js-sdk-version` in `mugon.toml` (a differing minor or patch version
only warns), and the first `<script>` of `index.html` should contain the SDK (otherwise it
warns, because large files then lose caching).

If the upload fails partway through, the preliminary version is automatically deleted.

**Requirements:**
- A `mugon.toml` must exist in the current directory.
- Credentials must be stored (run `mugon login` first) or `MUGON_PROJECT_API_KEY` must be set.
- The API key needs the `version_creator` role (plus `prerelease_tester` for a `PRERELEASE` game).
- `distribution-dir` must contain an `index.html` at its root.

---

### `mugon dev`

Starts a local development environment that mirrors how mugon.net hosts your game:

- A **parent frame** server (what mugon.net shows around your game) on the first free port from 3000.
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

### `mugon editor`

Runs the Mugon Editor: an alias for the `mugon-editor` binary, with all arguments passed through verbatim (`mugon editor` opens the scene editor, `mugon editor serve` is `mugon-editor serve`). The current directory is the game folder, so the editor picks up the `[editor]` table of your `mugon.toml`.

```sh
mugon editor [mugon-editor arguments...]
```

`mugon-editor` is looked up on `PATH`, then in `~/.mugon/editor` (where the editor's install script puts it). If it is not installed the command fails with a hint.

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

[[setting]]
key = "friendly-fire"
type = "boolean"
default = false
```

### Fields

| Field | Required | Default | Description |
|---|---|---|---|
| `id` | Yes | — | Unique project identifier. Must match the project ID on mugon.net. |
| `version` | Yes | — | Semver version string published when running `mugon publish`. |
| `js-sdk-version` | Yes | — | Version of the `@mugon/sdk` the game bundles. Its **major** is recorded with the published version and must match what the game actually bundles: `mugon publish` scans the distribution folder and fails on a major mismatch (a differing minor/patch only warns). |
| `distribution-dir` | No | `dist` | Directory containing the built game files to be uploaded. |
| `source-dir` | No | `src` | Source directory. Used as the default for `watch-paths`. |
| `watch-paths` | No | `[source-dir]` | Paths (files or directories) to watch for changes during `mugon dev`. |
| `[[setting]]` | No | none | Settings the game initiator can adjust before the game starts — see below. |
| `settings-valid-when` | No | — | Expression that must be true before the game can start — see below. |

### `[[setting]]` entries

Settings are declared per project and persisted with the version that `mugon publish` uploads. The initiator of a game adjusts them before the game starts, and the chosen values reach the game through the SDK (`context.settings`, or the typed `getBoolean`, `getNumber`, `getString`, `getStrings` helpers on the `Mugon.start` context).

Settings are **immutable once a version is published** — changing a declaration means publishing a new version. Render order follows declaration order, and `mugon dev` shows the same settings in a panel next to the game so they can be exercised locally.

```toml
[[setting]]
key = "friendly-fire"
label = "Friendly fire"
tab = "Gameplay"
type = "boolean"
default = false

[[setting]]
key = "round-time"
label = "Round time (s)"
tab = "Gameplay"
type = "number"
min = 30
max = 600
step = 30
default = 120
ui = "slider"

[[setting]]
key = "map"
type = "select"
options = ["desert", "forest"]
default = "desert"

[[setting]]
key = "modifiers"
type = "select"
multiple = true
options = [
    { value = "fast", label = "Fast rounds" },
    { value = "low_gravity", label = "Low gravity" },
]
default = ["fast"]

[[setting]]
key = "server.name"
type = "text"
default = "My server"
max-length = 32
```

| Field | Required | Default | Description |
|---|---|---|---|
| `key` | Yes | — | The key the game reads the value by. Lowercase alphanumeric segments separated by `.`, `_` or `-`. The `mugon.` namespace is reserved for the SDK's own keys. |
| `type` | Yes | — | `boolean`, `number`, `select` or `text`. |
| `default` | Yes | — | The value the setting starts at. Must match `type`; for `select` either a single string or a list of strings. |
| `label` | No | `key` | Display name in the settings UI. |
| `description` | No | `""` | Longer explanation shown alongside the input. |
| `tab` | No | `General` | Groups settings into tabs in the settings UI. |
| `ui` | No | — | Input to render. `boolean`: `switch`, `checkbox`. `number`: `slider`, `number-input`. `select`: `dropdown`, `segmented-control`, `radio-cards`. `text`: `text-input`, `textarea`. Omit to let the UI pick. |
| `min`, `max` | `number` only | — | Bounds of the allowed range. Both required for a number setting. |
| `step` | No | `1` | Increment of a number setting. The default must be reachable from `min` in whole steps. |
| `options` | `select` only | — | The choices, either bare strings (label = value) or `{ value = "...", label = "..." }` tables. |
| `multiple` | No | `false` | Whether a `select` setting takes more than one value. |
| `max-length` | No | `1024` | Maximum length of a `text` setting's value. |
| `enabled-when` | No | — | Expression that must be true for this setting to be editable — see below. |

#### Presetting values from the URL

Both `mugon dev` and mugon.net read starting values for settings from query parameters prefixed with `s.`, so a particular combination can be bookmarked or shared:

```
http://localhost:5173/?s.friendly-fire=true&s.round-time=180&s.map=forest
https://mugon.net/play?g=my-game&s.modifiers=fast,low_gravity
```

| Type | Accepted | Notes |
|---|---|---|
| `boolean` | `true`, `false`, `1`, `0` | Case insensitive. |
| `number` | Any number | Clamped into `[min, max]` and snapped to the nearest `step`. |
| `select` | `?s.k=a&s.k=b` or `?s.k=a,b` | Every value must be a declared option, otherwise the parameter is ignored. A single-choice select takes one value. |
| `text` | Any string | Truncated to `max-length`. |

A parameter that names an unknown setting, or carries a value that isn't valid for its type, is reported on the console and that setting stays at its default — one stale parameter in a shared link cannot make a game unstartable. The values are only a starting point: the initiator can still change them before starting. A peer that *joins* an existing game reads the parameters too, but they are never sent — a joining peer always plays with the values the host started with.

#### Gating settings with expressions

Two fields take an expression that must evaluate to a boolean:

| Field | Where | Meaning |
|---|---|---|
| `enabled-when` | on a `[[setting]]` | the initiator may change that setting only while it is true |
| `settings-valid-when` | top level | the game may only start while it is true |

```toml
settings-valid-when = 'settings["min-players"] < settings["max-players"]'

[[setting]]
key = "bots"
type = "boolean"
default = false

[[setting]]
key = "bot-count"
label = "Number of bots"
type = "number"
min = 1
max = 8
default = 4
enabled-when = 'settings["bots"]'
```

Expressions are a restricted subset of [CEL](https://cel.dev/), fully specified in [`specs/expressions/grammar.md`](../../specs/expressions/grammar.md). In short:

- Reference a setting as `settings["<key>"]`; that is the only variable in scope.
- Operators: `&&` `||` `!`, `==` `!=`, `<` `<=` `>` `>=`, `+` `-` `*` `/`, `in`, and `cond ? a : b`.
- One function: `size(x)`, for the length of a multi-select or a string.
- Nothing else — no macros, no type conversions, no string methods, no list literals.

Two behaviours worth knowing up front:

- **Every number setting is a `double`.** Comparisons accept plain integers (`settings["round-time"] >= 60`), but arithmetic needs a decimal point (`settings["round-time"] + 1.0`), because CEL has no `double + int` overload.
- **A disabled setting keeps its value**; only its input locks. A game should check the gating setting itself before reading a gated one.

Expressions are validated when you publish, against the settings that same version declares, so a typo, an undeclared key, a non-boolean result, or a type mismatch fails `mugon publish` with an explanation rather than misbehaving at play time. `mugon dev` evaluates them straight from `mugon.toml`, reporting problems in the browser console instead.

### `[editor]` table

Read by the Mugon Editor (`mugon editor`), ignored by every other command. All paths are relative to the project folder (absolute paths also work).

```toml
[editor.scenes]
json-path = "scenes"                  # where scenes (*.mugon-scene.json) are saved; default shown
bsn-path = "src/scenes"               # where exported BSN (<scene>.rs) goes; default shown
bsn-use = ["crate::components::*"]    # `use` lines in the exported BSN; default none

[editor.assets]
path = "assets"                       # "Export on save" copies the used assets here; default shown

[[editor.component]]
name = "Health"
fields = [{ name = "max", type = "f32", default = 100.0 }]
```

`[[editor.component]]` defines the components that can be attached to entities in the editor. Field `type` is one of `bool`, `i32`, `f32`, `string`, `vec2`, `vec3`, `color`, `enum`, `asset`; `enum` fields take `options` and an optional `rust_type`.

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

# Optional: override the API endpoint (useful for the staging environment or local dev)
mugon-net-api-url-override = "https://backend.stage.mugon.net/api/v1"
```

### Fields

| Field | Description |
|---|---|
| `credentials` | Map of project IDs to API keys. Populated by `mugon login`, cleared by `mugon logout`. |
| `mugon-net-api-url-override` | Overrides the default API base URL (`https://backend.mugon.net/api/v1`). |

The `[editor]` table (`library = "<folder>"`) holds the asset library folder of the Mugon Editor and is preserved by `mugon login` / `mugon logout`.

Credentials can also be supplied at runtime via the `MUGON_PROJECT_API_KEY` environment variable, which takes precedence over the stored value.

---

## License

[MIT](LICENSE-MIT)

## Links

- Repository and issues: <https://github.com/mugon-net/mugon-cli>
- JavaScript SDK: <https://github.com/mugon-net/mugon-sdk-js>
- Bevy SDK: <https://github.com/mugon-net/mugon-sdk-bevy>
- Lightyear transport: <https://github.com/mugon-net/mugon-sdk-lightyear>
- Platform: <https://mugon.net>
