<!-- 1-2 line summary of WHAT changed technically:
- Always link the relevant projects GitHub issue, unless it is a minor bugfix
- Good: "Modified FailoverDomain mapper to allow null ActiveClusterName #320"
- Bad: "Changed line 47 in mapper.go to add a nil check and updated the function signature" -->
**What changed?**


<!-- Your goal is to provide all the required context for a future maintainer 
to understand the reasons for making this change (see https://cbea.ms/git-commit/#why-not-how).
How did this work previously (and what was wrong with it)? What has changed, and why did you solve it 
this way?
- Good: "Active-active domains have independent cluster attributes per region. Previously,
  modifying cluster attributes required spedifying the default ActiveClusterName which
  updates the global domain default. This prevents operators from updating regional
  configurations without affecting the primary cluster designation. This change allows
  attribute updates to be independent of active cluster selection."
- Bad: "Improves domain handling" -->
**Why?**


<!-- Include specific test commands and setup. Please include the exact commands such that
another maintainer or contributor can reproduce the test steps taken. 
- e.g Unit test commands with exact invocation
  `go test -v ./common/types/mapper/proto -run TestFailoverDomainRequest`
- For integration tests include setup steps and test commands
  Example: "Started local server with `./cadence start`, then ran `make test_e2e`"
- For local simulation testing include setup steps for the server and how you ran the tests
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
- Always ensure that the description contains a link to the relevant GitHub issue.
- If this PR completes a feature users should know about, add release notes here
- Format: Brief user-facing description of what has been enabled with this feature -->
**Release notes**


<!-- Consider whether this change requires documentation updates in the Cadence-Docs repo
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
