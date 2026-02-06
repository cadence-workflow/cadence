# Epic Definition

An Epic represents a high-level initiative that delivers user-visible value or
meaningfully changes the system architecture or workflow.

Epics are used to:
- Make planning and prioritization transparent in OSS
- Provide context for related issues and PRs
- Enable better roadmap and release notes

---

## What qualifies as an Epic

An Epic typically:
- Spans multiple issues or PRs
- Requires a design decision or architectural discussion
- Benefits from visuals (diagrams, flows, screenshots)
- Takes more than a small incremental change to complete

Examples:
- Introducing a new subsystem or major refactor
- Changing public APIs or user workflows
- Cross-cutting improvements that affect multiple components
- Reliability, scalability, or operational initiatives

---

## What does NOT need an Epic

Do NOT create an Epic for:
- Small bug fixes or isolated improvements
- Refactors with no user-visible or architectural impact
- Pure cleanup, formatting, or mechanical changes
- Single-PR changes with limited scope

When in doubt, start with a normal issue and promote it to an Epic later.

---

## Required information for an Epic

Every Epic must include:
- A clear motivation or problem statement
- A link to a design document (RFC, PR, Google Doc, etc.)
- Visuals that help explain the architecture or flow
- Links to related issues or tasks

---

## Roadmap visibility

By default, Epics are tracked in the OSS roadmap.

Some Epics may be marked as **Hidden** for strategic or pre-launch reasons.
Hidden Epics are still tracked internally but are excluded from the public roadmap.

This distinction is intentional and explicit. 
