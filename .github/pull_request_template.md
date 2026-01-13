<!-- 1-2 line summary of WHAT changed technically:
- Focus on the key modification, not implementation details
- Good: "Modified FailoverDomain mapper to allow null ActiveClusterName"
- Bad: "Changed line 47 in mapper.go to add a nil check and updated the function signature" -->
**What changed?**


<!-- Explain WHY this change is needed. Provide full context and motivation:
- What problem does this solve? What's the use case?
- What's the impact if we don't make this change?
- Link related issues/discussions: "Fixes #1234" or "Related to discussion in #5678"
- Example: "Active-active domains have independent cluster attributes per region. Currently,
  modifying cluster attributes requires changing the ActiveClusterName, which incorrectly
  updates the global domain default. This prevents operators from updating regional
  configurations without affecting the primary cluster designation. This change allows
  attribute updates to be independent of active cluster selection."
- Bad: "This was needed" or "Improves domain handling" -->
**Why?**


<!-- Include specific test commands and setup:
- REQUIRED: Unit test commands with exact invocation
  Example: `go test -v ./common/types/mapper/proto -run TestFailoverDomainRequest`
- If integration tests: Include setup steps and test commands
  Example: "Started local server with `./cadence start`, then ran `make test_integration_matching`"
- If canary testing: Mention which canary, environment, and results
  Example: "Deployed to dev canary, monitored for 2 hours, no errors in logs"
- Good: Full commands that reviewers can copy-paste to verify
- Bad: "Tested locally" or "Added tests" -->
**How did you test it?**


<!-- Think about deployment risks:
- Backward/forward compatibility concerns?
- Performance impact?
- What could break in production?
- Safe to rollback?
- If truly N/A, you can mark it as such -->
**Potential risks**


<!-- Consider if this completes a user-facing feature:
- If this PR completes a feature users should know about, add release notes here
- Skip for: incremental work, internal refactoring, partial implementations
- Include for: completed features, breaking changes, major improvements
- Format: Brief user-facing description of what's now possible
- If unsure whether this completes a feature, you can skip -->
**Release notes**


<!-- Consider if this needs documentation updates:
- Config changes that affect operation-guide setup?
- New features needing user documentation?
- If yes: mention what needs updating (or link to docs PR in cadence-docs repo)
- If in doubt, add a note about potential doc needs
- Only mark N/A if you're certain no docs are affected -->
**Documentation Changes**


---

## Reviewer Validation

**PR Description Quality** (check these before reviewing code):

- [ ] **"What changed"** provides a clear 1-2 line summary
- [ ] **"Why"** explains the full motivation with sufficient context
- [ ] Project Issue is linked
- [ ] Release Issue is linked
- [ ] **Testing is documented:**
  - [ ] Unit test commands are included (with exact `go test` invocation)
  - [ ] Integration test setup/commands included (if integration tests were run)
  - [ ] Canary testing details included (if canary was mentioned)
- [ ] **Potential risks** section is thoughtfully filled out (or legitimately N/A)
- [ ] **Release notes** included if this completes a user-facing feature
- [ ] **Documentation** needs are addressed (or noted if uncertain)
