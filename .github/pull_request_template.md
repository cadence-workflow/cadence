<!-- 1-2 line summary of WHAT changed technically:
- Always link the relevant projects GitHub issue, unless it is a minor bugfix
- Good: "Modified FailoverDomain mapper to allow null ActiveClusterName #320"
- Bad: "added nil check" -->
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


<!-- If there are risks that the release engineer should know about document them here. 
For example:
- Has an API/IDL been modified? Is it backwards/forwards compatible? If not, what are the repecussions? 
- Has a schema change been introduced? Is it possible to roll back?
- Has a feature flag been re-used for a new purpose? 
- Is there a potential performance concern? Is the change modifying core task processing logic? 
- If truly N/A, you can mark it as such -->
**Potential risks**


<!-- If this PR completes a user facing feature or changes functionality add release notes here.
Your release notes should allow a user and the release engineer to understand the changes with little context.
Always ensure that the description contains a link to the relevant GitHub issue. -->
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
  - [ ] Project Issue is linked
- [ ] **"Why"** explains the full motivation with sufficient context
- [ ] **Testing is documented:**
  - [ ] Unit test commands are included (with exact `go test` invocation)
  - [ ] Integration test setup/commands included (if integration tests were run)
  - [ ] Canary testing details included (if canary was mentioned)
- [ ] **Potential risks** section is thoughtfully filled out (or legitimately N/A)
- [ ] **Release notes** included if this completes a user-facing feature
- [ ] **Documentation** needs are addressed (or noted if uncertain)
