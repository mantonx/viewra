# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Viewra project. ADRs document significant architectural decisions made during development.

## What is an ADR?

An Architecture Decision Record captures an important architectural decision made along with its context and consequences. This helps future developers understand not just *what* was decided, but *why* it was decided.

## ADR Format

Each ADR follows this template:

- **Title**: ADR-NNNN: Short descriptive title
- **Date**: When the decision was made
- **Status**: Proposed, Accepted, Deprecated, Superseded
- **Context**: The issue motivating this decision
- **Decision**: The change that we're proposing or have agreed to implement
- **Consequences**: What becomes easier or harder as a result

## Current ADRs

### Core Architecture
- [ADR-0001: Separation of Playback and Transcoding Modules](0001-module-separation.md) - Explains why we split transcoding from playback
- [ADR-0002: Service Registry Pattern](0002-service-registry-pattern.md) - Documents our inter-module communication pattern
- [ADR-0003: Two-Stage Transcoding Pipeline](0003-two-stage-transcoding-pipeline.md) - Details the encode→package pipeline architecture
- [ADR-0004: Clean Module Architecture](0004-clean-module-architecture.md) - Defines the standard module structure and patterns
- [ADR-0005: Playback and Transcoding Architecture](0005-playback-transcoding-architecture.md) - Details the playback decision and transcoding system

## Creating a New ADR

1. Copy the template below
2. Name it `NNNN-short-description.md` where NNNN is the next number
3. Fill in all sections
4. Update this README with a link to your ADR

## ADR Template

```markdown
# ADR-NNNN: [Title]

Date: YYYY-MM-DD
Status: Proposed/Accepted/Deprecated/Superseded

## Context

[Describe the issue motivating this decision]

## Decision

[Describe the change that we're proposing or have agreed to implement]

## Consequences

### Positive
- [Positive consequence 1]
- [Positive consequence 2]

### Negative
- [Negative consequence 1]
- [Negative consequence 2]

## Alternatives Considered

[What other options were considered and why were they rejected?]
```

## References

- [Architectural Decision Records](https://adr.github.io/)
- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)