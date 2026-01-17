# ViewRA Documentation

## Start Here

Choose your path based on what you need:

### 🤖 AI Assistants

1. [../CLAUDE.md](../CLAUDE.md) - Essential context, commands, rules
2. [PROJECT_STATUS.md](planning/PROJECT_STATUS.md) - Current state
3. [ARCHITECTURE.md](core/ARCHITECTURE.md) - System design (when needed)

### 👩‍💻 New Developers

1. [PROJECT_STATUS.md](planning/PROJECT_STATUS.md) - What exists, what's missing
2. [ARCHITECTURE.md](core/ARCHITECTURE.md) - How the system is organized
3. [CONVENTIONS.md](development/CONVENTIONS.md) - Code style and patterns
4. [decisions/README.md](decisions/README.md) - Why things are the way they are

### 🔧 Working on Features

| I want to... | Read this |
|--------------|-----------|
| Understand transcoding | [HLS Transcoding Guide](guides/HLS_TRANSCODING.md) |
| Build a plugin | [Plugin Development Guide](guides/PLUGIN_DEVELOPMENT.md) |
| Work on search | [Search Roadmap](planning/SEARCH_ROADMAP.md) |
| Fix technical debt | [Technical Debt](planning/TECHNICAL_DEBT.md) |

### 🚀 Operations / Deployment

- [Operations Guide](operations/README.md) - Deployment and runtime configuration
- [Environment Variables](operations/ENVIRONMENT_VARIABLES.md) - Complete config reference
- [Deployment Guide](operations/DEPLOYMENT.md) - Docker, systemd, Kubernetes
- [Troubleshooting](operations/TROUBLESHOOTING.md) - Common issues and solutions

---

## Quick Reference

### Project Status

| Document | Purpose |
|----------|---------|
| [PROJECT_STATUS.md](planning/PROJECT_STATUS.md) | Current state, metrics, recent work |
| [PROJECT_PLAN.md](planning/PROJECT_PLAN.md) | Roadmap and upcoming work |

### Architecture

| Document | Purpose |
|----------|---------|
| [ARCHITECTURE.md](core/ARCHITECTURE.md) | System design, layers, data flow |
| [decisions/](decisions/README.md) | 30 Architecture Decision Records |

### Development

| Document | Purpose |
|----------|---------|
| [CONVENTIONS.md](development/CONVENTIONS.md) | Go code style and patterns |
| [web/docs/CODING_STYLE.md](../web/docs/CODING_STYLE.md) | TypeScript/React conventions |
| [web/docs/COMPONENT_LIBRARY.md](../web/docs/COMPONENT_LIBRARY.md) | UI components |
| [web/docs/DESIGN_TOKENS.md](../web/docs/DESIGN_TOKENS.md) | Design system |

### Guides (How-To)

| Guide | Topic |
|-------|-------|
| [HLS Transcoding](guides/HLS_TRANSCODING.md) | Video streaming and transcoding |
| [Plugin Development](guides/PLUGIN_DEVELOPMENT.md) | Building enrichment plugins |
| [Enrichment Pipeline](guides/ENRICHMENT_PIPELINE.md) | Metadata enrichment system |
| [Subtitle Pipeline](guides/SUBTITLE_PIPELINE.md) | Subtitle extraction and delivery |
| [Real-Time Updates](guides/REAL_TIME_UPDATES.md) | SSE and Event Bus |

### Features (Deep Dives)

| Document | Topic |
|----------|-------|
| [Hardware Acceleration](features/HARDWARE_ACCELERATION.md) | GPU transcoding (NVENC, VAAPI, QSV) |
| [Tone Mapping](features/TONE_MAPPING.md) | HDR to SDR conversion |
| [features/README.md](features/README.md) | Full list and when to use features vs ADRs |

### Key ADRs

| ADR | Decision |
|-----|----------|
| [005](decisions/005-on-demand-transcoding-strategy.md) | On-demand transcoding strategy |
| [021](decisions/021-progressive-hls-transcoding.md) | Progressive HLS transcoding |
| [027](decisions/027-plugin-system-architecture.md) | Plugin system architecture |
| [028](decisions/028-user-authentication.md) | User authentication |

See [decisions/README.md](decisions/README.md) for all 30 ADRs.

### Operations

| Document | Purpose |
|----------|---------|
| [operations/README.md](operations/README.md) | Operations overview |
| [ENVIRONMENT_VARIABLES.md](operations/ENVIRONMENT_VARIABLES.md) | Complete config reference |
| [DEPLOYMENT.md](operations/DEPLOYMENT.md) | Docker, systemd, Kubernetes |
| [TROUBLESHOOTING.md](operations/TROUBLESHOOTING.md) | Common issues and solutions |

### Planning

| Document | Purpose |
|----------|---------|
| [TECHNICAL_DEBT.md](planning/TECHNICAL_DEBT.md) | Known debt and deferred improvements |
| [SEARCH_ROADMAP.md](planning/SEARCH_ROADMAP.md) | Remaining search improvements |
| [refactoring/](refactoring/) | Active refactoring plans |

### Archive

Completed planning and refactoring docs are in [archive/](archive/) for historical reference.

---

## Documentation Types

| Type | Location | Purpose |
|------|----------|---------|
| **ADRs** | `decisions/` | Record *why* decisions were made |
| **Guides** | `guides/` | Step-by-step *how to* implement |
| **Features** | `features/` | Deep technical *reference* |
| **Operations** | `operations/` | Deployment and runtime config |
| **Planning** | `planning/` | Roadmaps and active work |
| **Archive** | `archive/` | Completed/historical docs |
