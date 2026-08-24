# Ebitengine Boilerplate

A forkable starting point for cross-platform Ebitengine apps and games. It gives you a working local build pipeline first; deployment is a separate setup step you can tailor to your own release process.

## The two pipelines

### Primary: local build

`scripts/build.py` is the primary pipeline. Run it with no target flags and it will:

- inspect the local Go/toolchain environment;
- list every target, intended artifact path, local feasibility, and reason;
- build every target that is locally possible; and
- verify the artifacts it produced.

On Unix, `make build` is a convenient equivalent. On every platform, use:

```bash
python scripts/build.py --verify
```

Artifacts are written to `releases/{goos}/{goarch}/app/latest[.exe]` and are ignored by Git.

### Secondary: deploy

Deployment is intentionally not configured. The boilerplate includes an editor placeholder to make the next step visible, but it does not choose a store, GitHub Release, signing method, packaging format, credentials provider, or CI service for you.

When you are ready to ship, configure a deploy pipeline that consumes `releases/` and matches your product's destination and security requirements. See [ROADMAP.md](ROADMAP.md) for the planned extension points.

## Optional Ollama scaffolding

The `agentic-pipelines/` submodule, `pipeline.yaml`, and `api.sample.yaml` are optional examples for people who want to add Ollama-backed governance or automation. They are not part of the build pipeline and are never required for building or deploying.

If you choose to use them, copy `api.sample.yaml` to ignored `api.yaml`, configure your own local endpoint, and explicitly run the bootstrap scripts. Otherwise, you can ignore those files entirely.

## What you get

| Piece | Purpose |
| --- | --- |
| `cmd/app/main.go` | A single-file demo: drag and flick a bouncing "Hello, world." label. Delete or replace it with your app. |
| `scripts/build.py` | The canonical no-flag build pipeline and target-feasibility report. |
| `scripts/run_artifact.py` | Runs a built host artifact or lists all artifacts. |
| `Makefile` | Unix convenience entry point with project-local Go caches. |
| `releases/` | Ignored build output. |
| `ROADMAP.md` | Build, deploy, and future-target extension plan. |

## Quick start

1. Fork or clone the repository and rename it.
2. Install Go 1.26.4 and Python 3.10+.
3. Build and verify every locally feasible target:

   ```bash
   python scripts/build.py --verify
   ```

4. Run the built host artifact:

   ```bash
   python scripts/run_artifact.py
   ```

The current demo is self-contained in `cmd/app/main.go`; replacing that one file is enough to begin your own app.

## Current target policy

| Target | Status |
| --- | --- |
| Native host desktop | Attempted and verified by the actual local build. |
| JS/WASM | Built when the Go toolchain supports it; browser serving/packaging is left to you. |
| Non-host desktop | Reported with a reason until cross-CGO toolchain support is deliberately added. |
| Android | Deferred until gomobile and NDK integration are configured. |

## Project layout

```text
cmd/app/main.go              Disposable Ebitengine demo application
scripts/build.py             Primary no-flag build pipeline
scripts/run_artifact.py      Artifact discovery and local execution
releases/                    Ignored generated artifacts
agentic-pipelines/           Optional Ollama/governance scaffolding
pipeline.yaml                Optional governance demonstration contract
ROADMAP.md                   Future build and deploy work
AGENTS.md                    Instructions for coding agents
```

## Contributing

Fork it and make it yours. If you add a useful target, packaging workflow, or deploy integration, contributions back to the boilerplate are welcome.
