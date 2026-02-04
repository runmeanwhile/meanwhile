# Meanwhile Product Restructure Plan

This plan covers all changes needed to transition Meanwhile from a personal MIT-licensed project to a properly structured open-core product with clear OSS and enterprise boundaries.

---

## 1. GitHub Organization & Module Path

### Current
- Repo: `github.com/runmeanwhile/meanwhile`
- Module: `github.com/runmeanwhile/meanwhile`

### Target
- Org: `github.com/runmeanwhile` (may rename to `meanwhile` if acquired)
- Repo: `github.com/runmeanwhile/meanwhile`
- Module: `github.com/runmeanwhile/meanwhile`

### Why not vanity import (for now)?

Vanity imports (e.g., `runmeanwhile.com/meanwhile`) add complexity:
- Need to set up and maintain redirect infrastructure
- Extra moving part before validating anyone uses the package

**Revisit later** if:
- The package gets real adoption as a Go library
- You acquire the `meanwhile` org and want to rename without breaking imports

### Tasks
- [x] Create GitHub org `runmeanwhile` (free tier)
- [ ] Transfer repo to org (or create fresh and migrate)
- [ ] Update `go.mod` module path to `github.com/runmeanwhile/meanwhile`
- [ ] Update all internal imports in `pkg/`, `examples/`, `studio/`
- [ ] Set up GitHub org profile README
- [ ] Configure org-level settings (security policy, contributing guidelines)

### If `meanwhile` org acquired later
- [ ] Rename org `runmeanwhile` → `meanwhile`
- [ ] Keep `runmeanwhile` org as placeholder (prevents hijacking)
- [ ] Update `go.mod` to `github.com/meanwhile/meanwhile`
- [ ] Update all imports (breaking change, requires new major version)
- [ ] Old imports continue to work via GitHub redirects (as long as placeholder org exists)

### Import Update Scope
```
pkg/**/*.go           - ~50+ files
examples/**/*.go      - 24 examples
studio/**/*.go        - future files
```

### go.mod Change
```go
// Before
module github.com/runmeanwhile/meanwhile

// After
module github.com/runmeanwhile/meanwhile
```

---

## 2. Licensing

### Current
- MIT License (fully permissive)

### Target
- **Runtime + CLI + Studio OSS**: Apache 2.0 (true open source)
- **Studio Enterprise (`ee/`)**: Proprietary (Meanwhile Enterprise License)

### Why Apache 2.0 over BSL

1. **Lower friction**: No license anxiety for early adopters
2. **Community goodwill**: "Fully open source" is a better story at this stage
3. **Patent protection**: Apache 2.0 includes patent grants (MIT doesn't)
4. **Temporal model**: They're MIT-licensed, make money from cloud/enterprise
5. **Premature to restrict**: Validate first, protect later if needed

### Why Apache 2.0 over MIT

Apache 2.0 adds:
- Explicit patent license (protects users from patent claims)
- Contribution terms built-in
- Trademark protection clause
- Better for corporate adoption

### Monetization Strategy (without BSL)

Revenue comes from:
- **Enterprise features** (`ee/`): SSO, teams, RBAC, audit, Postgres
- **Managed cloud** (future): Hosted Meanwhile
- **Support contracts**: Priority support, SLAs

### Future Option: Relicense if Needed

If the project gets significant traction and cloud providers start offering "Managed Meanwhile":
- CLA gives you the right to relicense future versions
- Can move to BSL for new major versions (controversial but possible)
- By then you'll have leverage and it won't look defensive

### Tasks
- [ ] Replace `LICENSE` with Apache 2.0 text
- [ ] Create `licenses/` directory with:
  - `licenses/APACHE-2.0.md` - Full Apache text
  - `licenses/MEANWHILE-ENTERPRISE.md` - Proprietary license for `ee/`
- [ ] Add license headers to source files (optional but recommended)
- [ ] Update `README.md` license section
- [ ] Update `CONTRIBUTING.md` with DCO requirement (lighter than CLA)
- [ ] Add `LICENSE` file in `studio/ee/` with proprietary terms

### License Header (optional)
```go
// Copyright 2026 Meanwhile
// SPDX-License-Identifier: Apache-2.0
```

### DCO (Developer Certificate of Origin)

Lighter than a CLA. Contributors sign off commits with:
```
Signed-off-by: Name <email@example.com>
```

This certifies they have the right to submit the code. Use `git commit -s` to add automatically.

Add to `CONTRIBUTING.md`:
```markdown
## Developer Certificate of Origin

By contributing to this project, you certify that your contributions
are made under the Apache 2.0 license and that you have the right to
submit them. Please sign off your commits:

    git commit -s -m "Your commit message"
```

---

## 3. Trademark

### Tasks
- [ ] Register "Meanwhile" trademark with USPTO (Class 9 - software, Class 42 - SaaS)
- [ ] Create `TRADEMARK.md` with usage guidelines
- [ ] Add trademark notice to README footer

### Trademark Notice
```
Meanwhile is a trademark of Meanwhile, Inc.
The Meanwhile logo and name may not be used without permission.
```

---

## 4. Domain & Branding

### Decisions
- **Domain**: `runmeanwhile.com`
- **Tagline**: "Meanwhile is a collaboration runtime for AI agents"
- **CLI binary**: `meanwhile`
- **Module path**: `github.com/runmeanwhile/meanwhile` (standard GitHub path)

### Tasks
- [ ] Register `runmeanwhile.com`
- [ ] Set up DNS (Cloudflare recommended)
- [ ] Create landing page placeholder
- [ ] Update all docs to use new domain
- [ ] Set up email: `hello@runmeanwhile.com`, `security@runmeanwhile.com`

### Future: Vanity Import (if needed)

If the Go package gets real adoption and you want cleaner imports or need to rename the org:
- Set up `runmeanwhile.com/meanwhile` → GitHub redirect
- Update `go.mod` to vanity path
- See https://go.dev/ref/mod#serving-from-custom-domain

---

## 5. Repository Structure

### Current
```
meanwhile/
├── pkg/                    # All runtime packages
├── examples/               # Example code
├── docs/                   # Documentation
├── studio/                 # Just PRODUCT_PLAN.md
├── LICENSE                 # MIT
├── README.md
├── CHANGELOG.md
└── ...
```

### Target
```
meanwhile/
├── pkg/                    # Runtime packages (Apache 2.0)
│   ├── agent/
│   ├── engine/
│   ├── protocol/
│   ├── provider/
│   ├── tool/
│   ├── memory/
│   ├── mcp/
│   ├── integration/
│   ├── scheduler/
│   ├── requestregistry/
│   └── ...
│
├── cli/                    # CLI application (Apache 2.0) [NEW]
│   ├── cmd/
│   │   └── meanwhile/
│   │       └── main.go
│   ├── internal/
│   │   ├── run/            # `meanwhile run` command
│   │   ├── start/          # `meanwhile start` command
│   │   ├── version/        # `meanwhile version`
│   │   └── config/         # CLI config handling
│   └── README.md
│
├── studio/                 # Web UI (Apache 2.0 + Proprietary) [RESTRUCTURE]
│   ├── backend/            # Go backend (Apache 2.0)
│   │   ├── server/         # HTTP server, API routes
│   │   ├── store/          # Persistence (SQLite default)
│   │   ├── runner/         # Session runner
│   │   ├── persona/        # PersonaSpec compiler
│   │   └── ...
│   ├── frontend/           # Next.js app (Apache 2.0)
│   │   ├── app/
│   │   ├── components/
│   │   └── ...
│   ├── ee/                 # Enterprise features [PROPRIETARY]
│   │   ├── LICENSE         # Meanwhile Enterprise License
│   │   ├── sso/            # SSO/SAML
│   │   ├── audit/          # Audit logging
│   │   ├── teams/          # Team management
│   │   ├── rbac/           # Role-based access
│   │   └── postgres/       # Postgres adapter (SQLite is OSS)
│   └── README.md
│
├── examples/               # Example code
├── docs/                   # Documentation
│   ├── guides/
│   ├── concepts/
│   ├── recipes/
│   └── api/                # [NEW] Generated API docs
│
├── licenses/               # [NEW] License texts
│   ├── APACHE-2.0.md
│   └── MEANWHILE-ENTERPRISE.md
│
├── .github/
│   ├── workflows/
│   ├── ISSUE_TEMPLATE/
│   └── PULL_REQUEST_TEMPLATE.md
│
├── LICENSE                 # Apache 2.0 (root)
├── TRADEMARK.md            # [NEW]
├── README.md
├── CHANGELOG.md
├── CONTRIBUTING.md
├── SECURITY.md
├── CODE_OF_CONDUCT.md
├── go.mod
├── go.sum
└── Makefile
```

### Tasks
- [ ] Create `cli/` directory structure
- [ ] Create `cli/cmd/meanwhile/main.go` entry point
- [ ] Implement `meanwhile version` command
- [ ] Implement `meanwhile run <config>` command (headless execution)
- [ ] Implement `meanwhile start` command (launches studio)
- [ ] Restructure `studio/` into `backend/` and `frontend/`
- [ ] Create `studio/ee/` directory with proprietary LICENSE
- [ ] Create `licenses/` directory with all license texts
- [ ] Create `TRADEMARK.md`
- [ ] Update `.gitignore` for new structure

---

## 6. CLI Design

### Commands
```bash
meanwhile version                    # Print version info
meanwhile run <config.yaml>          # Run session headless, output JSON
meanwhile run --protocol brainstorm  # Run with inline protocol
meanwhile start                      # Start Studio (web UI)
meanwhile start --port 3000          # Custom port
meanwhile init                       # Initialize ~/.meanwhile/ config
meanwhile personas list              # List configured personas
meanwhile personas create            # Interactive persona creation
```

### Config Location
```
~/.meanwhile/
├── config.yaml           # Global config (API keys, defaults)
├── personas/             # Persona definitions
│   ├── skeptic.yaml
│   └── marketer.yaml
├── presets/              # Session presets
└── studio.db             # SQLite database (Studio state)
```

### Tasks
- [ ] Design CLI command structure
- [ ] Implement cobra/urfave CLI framework
- [ ] Add `--output json|yaml|text` flag for `run`
- [ ] Add `--verbose` / `--quiet` flags
- [ ] Create man pages / shell completions

---

## 7. Studio Architecture

### OSS Features (Apache 2.0)
- Persona editor (create, edit, version)
- Protocol runner (select, configure, run)
- Session timeline (live events, messages)
- SQLite persistence
- Single-user (no auth)
- Local MCP connections
- Human-in-the-loop (local only)

### Enterprise Features (Proprietary `ee/`)
- SSO/SAML authentication
- Team management & sharing
- Role-based access control (RBAC)
- Audit logging & compliance
- Postgres persistence
- Multi-user workspaces
- Priority support

### Feature Gating Approach
```go
// studio/backend/features/features.go
package features

import "os"

func IsEnterpriseEnabled() bool {
    // Check for license key or build tag
    return os.Getenv("MEANWHILE_LICENSE_KEY") != "" ||
           enterpriseBuildTag
}

func RequireEnterprise(feature string) error {
    if !IsEnterpriseEnabled() {
        return fmt.Errorf("%s requires Meanwhile Enterprise", feature)
    }
    return nil
}
```

### Tasks
- [ ] Define clear OSS vs Enterprise feature boundary
- [ ] Implement feature detection (`features.IsEnterpriseEnabled()`)
- [ ] Create Studio backend scaffold
- [ ] Create Studio frontend scaffold (Next.js)
- [ ] Implement SQLite store (OSS)
- [ ] Implement Postgres store (Enterprise, in `ee/`)
- [ ] Add SSO integration (Enterprise, in `ee/`)

---

## 8. Changelog Structure

### Current Format
Standard keep-a-changelog, no edition tags.

### Target Format
```markdown
# Changelog

## [Unreleased]

### Runtime
- Added WebSocket support for real-time events
- Fixed memory leak in long-running sessions

### CLI
- New `meanwhile personas` subcommands
- Added shell completion generation

### Studio
- New persona editor with live preview
- SQLite performance improvements

### Studio Enterprise
- SSO/SAML integration
- Audit logging for compliance
- Team persona sharing

## [0.2.0] - 2026-03-01

### Runtime
...
```

### Rules
1. Always tag entries by component (Runtime, CLI, Studio, Studio Enterprise)
2. If a release is enterprise-only, say so explicitly
3. OSS users should be able to filter mentally by ignoring "Studio Enterprise"
4. Don't release versions that are 100% enterprise changes (batch with OSS changes)

### Tasks
- [ ] Update `CHANGELOG.md` with new format
- [ ] Add changelog format to `CONTRIBUTING.md`
- [ ] Consider changelog automation (release-please, semantic-release)

---

## 9. Documentation Updates

### Files to Update
- [ ] `README.md` - New tagline, license badge, install instructions, CLI examples
- [ ] `CONTRIBUTING.md` - CLA/DCO, enterprise contribution policy
- [ ] `SECURITY.md` - Update contact to `security@runmeanwhile.com`
- [ ] `docs/overview.md` - Position as "collaboration runtime"
- [ ] `docs/architecture.md` - Add CLI and Studio sections

### New Files
- [ ] `docs/guides/quickstart.md` - 5-minute getting started
- [ ] `docs/guides/cli.md` - CLI reference
- [ ] `docs/guides/studio.md` - Studio user guide
- [ ] `docs/guides/enterprise.md` - Enterprise features overview
- [ ] `TRADEMARK.md` - Trademark usage policy

### README Updates
```markdown
# Meanwhile

**A collaboration runtime for AI agents.**

Meanwhile enables structured multi-perspective deliberation. Define protocols,
assemble teams, and run collaboration sessions that produce better outcomes
than single-agent approaches.

## Quick Start

```bash
# Install CLI
go install github.com/runmeanwhile/meanwhile/cli/cmd/meanwhile@latest

# Run a brainstorm session
meanwhile run --protocol brainstorm --topic "Product naming strategy"

# Or start the Studio UI
meanwhile start
# Open http://localhost:3000
```

## Use as a library

```bash
go get github.com/runmeanwhile/meanwhile
```

```go
import "github.com/runmeanwhile/meanwhile/pkg/engine"
```

## License

Meanwhile is open source under the [Apache License 2.0](LICENSE).

Enterprise features (SSO, teams, audit logging) require a
[commercial license](https://runmeanwhile.com/enterprise).
```

---

## 10. CI/CD Updates

### Current
- GitHub Actions for test, lint

### Additional Workflows
- [ ] Add CLI build workflow (multi-platform binaries)
- [ ] Add Studio build workflow (Docker image)
- [ ] Add release workflow (goreleaser)
- [ ] Add license header check
- [ ] Add enterprise feature isolation check (ensure `ee/` imports are gated)

### Release Artifacts
```
meanwhile_0.2.0_darwin_amd64.tar.gz
meanwhile_0.2.0_darwin_arm64.tar.gz
meanwhile_0.2.0_linux_amd64.tar.gz
meanwhile_0.2.0_linux_arm64.tar.gz
meanwhile_0.2.0_windows_amd64.zip
meanwhile-studio:0.2.0  (Docker image)
```

---

## 11. Migration Checklist

### Phase 1: Foundation (Do First)
- [ ] Register `runmeanwhile.com` domain
- [x] Create GitHub org `runmeanwhile`
- [ ] Transfer/migrate repo to `runmeanwhile/meanwhile`
- [ ] Update module path in `go.mod` to `github.com/runmeanwhile/meanwhile`
- [ ] Update all imports
- [ ] Replace LICENSE with Apache 2.0
- [ ] Create `licenses/` directory
- [ ] Update README with new license info
- [ ] Tag release v0.2.0 with new structure

### If `meanwhile` org acquired (anytime after Phase 1)
- [ ] Rename GitHub org `runmeanwhile` → `meanwhile`
- [ ] Create new `runmeanwhile` org as placeholder (reserves name, prevents hijacking)
- [ ] Update module path in `go.mod` to `github.com/meanwhile/meanwhile`
- [ ] Update all imports (breaking change for library users)
- [ ] GitHub redirects keep git operations working
- [ ] Optionally set up vanity import at that point to prevent future churn

### Phase 2: CLI
- [ ] Create `cli/` directory structure
- [ ] Implement `meanwhile version`
- [ ] Implement `meanwhile run`
- [ ] Implement `meanwhile start` (placeholder, launches message)
- [ ] Add to release workflow
- [ ] Document in README

### Phase 3: Studio OSS
- [ ] Create `studio/backend/` scaffold
- [ ] Create `studio/frontend/` scaffold
- [ ] Implement SQLite store
- [ ] Implement basic persona editor
- [ ] Implement protocol runner
- [ ] Implement session timeline
- [ ] Connect `meanwhile start` to Studio

### Phase 4: Studio Enterprise
- [ ] Create `studio/ee/` with proprietary LICENSE
- [ ] Implement feature gating
- [ ] Implement Postgres store
- [ ] Implement SSO/SAML
- [ ] Implement audit logging
- [ ] Create enterprise docs
- [ ] Set up license key infrastructure

### Phase 5: Polish
- [ ] Register trademark
- [ ] Create `TRADEMARK.md`
- [ ] Finalize landing page
- [ ] Set up billing/licensing system
- [ ] Announce launch

---

## 12. Open Questions

1. **Incorporation**: When to incorporate (Meanwhile, Inc.)? Needed for enterprise contracts, trademark registration, and if you later want to offer a CLA.

2. **CLA vs DCO**: Do you want contributors to sign a CLA (gives you flexibility to relicense) or use DCO (lighter weight, sign-off only)?

3. **Enterprise pricing**: What's the pricing model? Per-seat? Per-org? Usage-based?

4. **Studio tech stack**: Confirmed Next.js + Go? Or consider alternatives (e.g., Remix, SvelteKit)?

5. **Release cadence**: Monthly? When ready? How do you handle enterprise-only releases?

6. **Sub-packages**: Do you want users to import sub-packages like `github.com/runmeanwhile/meanwhile/pkg/engine` or flatten to `github.com/runmeanwhile/meanwhile/engine`? Current structure uses `pkg/` prefix.

---

## Summary

| Area | Current | Target |
|------|---------|--------|
| **Org** | `runmeanwhile` | `runmeanwhile` (may rename to `meanwhile` later) |
| **Module** | `github.com/runmeanwhile/meanwhile` | `github.com/runmeanwhile/meanwhile` |
| **License** | MIT | Apache 2.0 (runtime, CLI, studio OSS) + Proprietary (ee/) |
| **Domain** | none | `runmeanwhile.com` |
| **Tagline** | "multi-agent framework" | "collaboration runtime for AI agents" |
| **CLI** | none | `meanwhile run`, `meanwhile start` |
| **Studio** | planned | OSS (SQLite, single-user) + Enterprise (Postgres, teams, SSO) |
| **Changelog** | flat | tagged by component (Runtime, CLI, Studio, Enterprise) |
