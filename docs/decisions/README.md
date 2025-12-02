# Architecture Decision Records (ADRs)

**Documenting significant technical and architectural decisions**

---

## What is an ADR?

An Architecture Decision Record captures a significant decision, the context behind it, alternatives considered, and consequences (both positive and negative). ADRs help future developers understand WHY choices were made, not just WHAT was implemented.

### When to Create an ADR

Create an ADR when making decisions about:
- System architecture or design patterns
- Technology choices (databases, frameworks, libraries)
- Data models or schemas
- API design
- Performance or scalability approaches
- Security or privacy implementations

**Don't create an ADR for:**
- Implementation details within an established pattern
- Bug fixes
- Refactoring that doesn't change approach
- Minor dependency updates

---

## ADR Index

| # | Title | Status | Date | Tags |
|---|-------|--------|------|------|
| [005](005-on-demand-transcoding-strategy.md) | On-Demand Transcoding Strategy | Accepted | 2025-11-XX | transcoding, streaming |
| [006](006-image-handling-strategy.md) | Image Handling Strategy | Accepted | 2025-11-XX | images, caching |
| [007](007-unified-task-scheduler.md) | Unified Task Scheduler | Accepted | 2025-11-XX | scheduler, background-jobs |
| [008](008-music-artist-artwork-extraction.md) | Music Artist Artwork Extraction | Accepted | 2025-11-XX | music, images |
| [009](009-migrate-transcode-cleanup-to-unified-scheduler.md) | Migrate Transcode Cleanup to Unified Scheduler | Accepted | 2025-11-XX | scheduler, cleanup |
| [010](010-container-refactoring-strategy.md) | Container Refactoring Strategy | Accepted | 2025-11-XX | architecture, refactoring |
| [011](011-architectural-improvements-phase-1.md) | Architectural Improvements Phase 1 | Accepted | 2025-11-XX | architecture, testing |
| [012](012-music-database-architecture.md) | Music Database Architecture | Accepted | 2025-11-XX | database, music |
| [013](013-library-browsing-ux-improvements.md) | Library Browsing UX Improvements | Accepted | 2025-11-XX | frontend, ux |
| [014](014-library-scanner-resilience-improvements.md) | Library Scanner Resilience Improvements | Accepted | 2025-11-XX | scanner, reliability |
| [015](015-player-enhancement-strategy.md) | Player Enhancement Strategy | Accepted | 2025-11-XX | player, frontend |
| [016](016-seek-position-transcoding.md) | Seek Position Transcoding for HLS | Proposed | 2025-11-XX | transcoding, seeking |
| [018](018-infinite-scroll-image-loading-architecture.md) | Infinite Scroll Image Loading Architecture | Accepted | 2025-11-XX | frontend, images, performance |
| [019](019-watch-progress-tracking-reliability.md) | Watch Progress Tracking Reliability | Accepted | 2025-11-XX | progress, backend |
| [020](020-segment-based-on-demand-transcoding.md) | Segment-Based On-Demand Transcoding | **REJECTED** | 2025-01-20 | transcoding, streaming |
| [021](021-progressive-hls-transcoding.md) | Progressive HLS Transcoding (Jellyfin-Style) | Proposed | 2025-11-XX | transcoding, streaming |
| [022](022-library-package-refactoring.md) | Library Package Refactoring and Simplification | Proposed | 2025-11-22 | architecture, refactoring, complexity |
| [025](025-resilient-library-scanner-v2.md) | Resilient Library Scanner V2 - Checkpoint Recovery | Accepted | 2025-11-22 | scanner, reliability |
| [026](026-app-restructuring-and-auth.md) | App Package Restructuring | Proposed | 2025-12-02 | architecture, refactoring |
| [027](027-plugin-system-architecture.md) | Plugin System Architecture | Proposed | 2025-12-02 | plugins, metadata, extensibility |
| [028](028-user-authentication.md) | User Authentication | Proposed | 2025-12-02 | auth, security, users |
| [029](029-settings-infrastructure.md) | Settings Infrastructure | Proposed | 2025-12-02 | settings, configuration |
| [030](030-multi-language-audio-subtitles.md) | Multi-Language Audio & Subtitles | Proposed | 2025-11-26 | playback, subtitles, audio |
| [031](031-design-system-improvements.md) | Design System Improvements | Proposed | 2025-12-02 | frontend, design, ux |

---

## ADR Lifecycle

### Statuses

- **Proposed** - Decision is being considered, not yet implemented
- **Accepted** - Decision has been made and implemented
- **Deprecated** - Decision is no longer recommended (but may still be in use)
- **Superseded by ADR-XXX** - Decision has been replaced by a newer one
- **Rejected** - Decision was considered but NOT implemented (document why!)

### Evolution

ADRs are **immutable once accepted**. If you need to change a decision:

1. Create a new ADR documenting the new decision
2. Update the old ADR status to "Superseded by ADR-XXX"
3. Reference the old ADR in the new one's context

**Keep rejected ADRs!** They document what was tried and why it didn't work, preventing future teams from making the same mistakes.

---

## ADR Template

```markdown
# ADR NNN: [Short Descriptive Title]

## Status

[Proposed | Accepted | Deprecated | Superseded by ADR-XXX | Rejected]

## Context

What is the issue or situation that requires a decision?
- What problem are we solving?
- What constraints do we have?
- What are the current pain points?

## Decision

What is the change we're proposing or making?
- Be specific about what will be done
- Include key implementation details

## Consequences

### Positive
- What becomes easier, faster, or better?
- What problems does this solve?

### Negative
- What becomes harder or more complex?
- What trade-offs are we making?

### Neutral
- What other implications should we be aware of?

## Alternatives Considered

What other options did we evaluate?
- For each alternative: what it was and why we didn't choose it
- This helps future developers understand the decision space

## References

- Related ADRs
- External resources (blog posts, documentation, RFCs)
- Code references (PRs, commits, files)
- Discussion threads or issues
```

---

## Creating a New ADR

### 1. Find the Next Number

Check the index above for the next available number. Currently: **032**

### 2. Create the File

```bash
# From project root
touch docs/decisions/022-your-decision-name.md
```

### 3. Use the Template

Copy the template above and fill in each section thoughtfully.

### 4. Update This Index

Add your ADR to the table above with:
- Number
- Title (link to file)
- Status (usually "Proposed" initially)
- Date
- Relevant tags

### 5. Commit

```bash
git add docs/decisions/022-your-decision-name.md
git commit -m "docs: Add ADR 022 - Your Decision Name"
```

---

## Naming Convention

**Format**: `NNN-descriptive-name-with-dashes.md`

- **NNN**: Three-digit number (e.g., 005, 022, 123)
- **descriptive-name**: Lowercase, words separated by dashes
- **Extension**: `.md`

**Examples**:
- ✅ `022-websocket-streaming-architecture.md`
- ✅ `023-offline-caching-strategy.md`
- ❌ `ADR-022-name.md` (old format, being phased out)
- ❌ `22-name.md` (use three digits)
- ❌ `022_name.md` (use dashes, not underscores)

---

## Tips for Writing Good ADRs

### Be Concise

ADRs should be readable in 5-10 minutes. Focus on:
- **Why** the decision was made
- **What** the key trade-offs are
- **What alternatives** were considered

Skip:
- Exhaustive implementation details (put those in code comments or docs)
- Obvious consequences
- Excessive background (link to external resources instead)

### Document Rejected Decisions

Some of the most valuable ADRs are REJECTED ones:
- ADR 020: Segment-Based Transcoding (REJECTED) - Documents what didn't work and why
- This prevents future teams from trying the same approach

### Reference Liberally

Link to:
- Other ADRs that provide context
- External documentation that influenced the decision
- PRs or commits that implement the decision
- Issues or discussions that led to the decision

### Update Status

When a decision is superseded or deprecated:
1. Update the status in the ADR file
2. Add a note at the top explaining what superseded it
3. Update the index table above
4. **Don't delete the old ADR** - it's still valuable history

---

## FAQ

**Q: Should I create an ADR for every decision?**
A: No. Create ADRs for *significant* decisions that future developers will need to understand. Use your judgment: if someone in 6 months asks "why did we do it this way?", that's a good candidate for an ADR.

**Q: Can I update an accepted ADR?**
A: Only to fix typos or clarify language. If the decision itself changes, create a new ADR and mark the old one as superseded.

**Q: What if I'm not sure about the decision yet?**
A: Create a "Proposed" ADR! This is a great way to document your thinking and get feedback before committing.

**Q: How detailed should the "Alternatives Considered" section be?**
A: Enough to show you did your homework. 2-3 sentences per alternative is usually enough. Focus on *why* they weren't chosen.

---

**Last Updated**: December 2, 2025
