const test = require("node:test");
const assert = require("node:assert/strict");
const {
  parseSections,
  findMissingSections,
  normalize,
  readTemplateHeadings,
} = require("./validate-issue.js");

const bugRequired = readTemplateHeadings("bug_report.md");
const featureRequired = readTemplateHeadings("feature_request.md");

test("required headings are read from the issue templates", () => {
  assert.deepEqual(bugRequired, [
    "Description",
    "Steps to Reproduce / How to Trigger",
    "Expected Behavior",
    "Actual Behavior",
    "Logs / Screenshots",
    "Environment",
  ]);
  assert.deepEqual(featureRequired, [
    "Description",
    "Is this a breaking change?",
    "Scope of the feature (server, specific client, all clients)",
  ]);
});

test("colon-ending prose stays in the required section", () => {
  const body = [
    "### Description",
    "Matching stops dispatching tasks.",
    "### Actual Behavior",
    "The server logs this warning periodically:",
    "cadence-history keeps retrying the task list.",
    "### Expected Behavior",
    "The warning is not emitted.",
  ].join("\n");

  const actual = parseSections(body, bugRequired).get(
    normalize("Actual Behavior")
  );
  assert.match(actual, /The server logs this warning periodically:/);
  assert.match(actual, /cadence-history keeps retrying the task list/);

  const missing = findMissingSections(body, bugRequired);
  assert.equal(missing.includes("Actual Behavior"), false);
});

test("fenced yaml, shell, and markdown lines stay in the section", () => {
  const body = [
    "### Logs / Screenshots",
    "```yaml",
    "persistence:",
    "  defaultStore: cassandra",
    "dynamicconfig:",
    "```",
    "```bash",
    "#!/bin/bash",
    "# restart the service",
    "```",
    "```markdown",
    "### Environment",
    "fenced environment",
    "### Actual Behavior",
    "fenced actual behavior",
    "**Description**",
    "```",
    "Visible log summary.",
    "### Environment",
    "Prod cluster.",
  ].join("\n");

  const sections = parseSections(body, bugRequired);
  const logs = sections.get(normalize("Logs / Screenshots"));
  assert.match(logs, /persistence:/);
  assert.match(logs, /defaultStore: cassandra/);
  assert.match(logs, /#!\/bin\/bash/);
  assert.match(logs, /# restart the service/);
  assert.match(logs, /### Environment/);
  assert.match(logs, /fenced environment/);
  assert.match(logs, /### Actual Behavior/);
  assert.match(logs, /fenced actual behavior/);
  assert.match(logs, /\*\*Description\*\*/);
  assert.match(logs, /Visible log summary/);
  assert.equal(sections.get(normalize("Environment")), "Prod cluster.");

  const missing = findMissingSections(body, bugRequired);
  assert.equal(missing.includes("Logs / Screenshots"), false);
  assert.equal(missing.includes("Environment"), false);
  assert.equal(missing.includes("Actual Behavior"), true);
});

test("required template headings flush the previous section", () => {
  const bugBody = [
    "### Description",
    "First section body.",
    "## Expected behavior",
    "The workflow completes.",
    "### steps to reproduce / how to trigger",
    "1. Start the server.",
  ].join("\n");

  const bugSections = parseSections(bugBody, bugRequired);
  assert.equal(
    bugSections.get(normalize("Description")),
    "First section body."
  );
  assert.equal(
    bugSections.get(normalize("Expected Behavior")),
    "The workflow completes."
  );
  assert.equal(
    bugSections.get(normalize("Steps to Reproduce / How to Trigger")),
    "1. Start the server."
  );

  const featureBody = [
    "### Description",
    "Add a metric.",
    "### Is this a breaking change?",
    "No.",
    "### Scope of the feature (server, specific client, all clients)",
    "Server only.",
  ].join("\n");
  const featureSections = parseSections(featureBody, featureRequired);
  assert.equal(featureSections.get(normalize("Description")), "Add a metric.");
  assert.equal(
    featureSections.get(normalize("Is this a breaking change?")),
    "No."
  );
  assert.equal(
    featureSections.get(
      normalize("Scope of the feature (server, specific client, all clients)")
    ),
    "Server only."
  );
});

test("non-template headings and bold lines stay content while empty sections stay missing", () => {
  const body = [
    "### Description",
    "Workers stop polling.",
    "### Suggested fix",
    "Heading text stays in the description.",
    "**Note**",
    "Bold text stays in the description.",
    "**If yes, explain why:**",
    "This prompt stays in the description.",
    "### Expected Behavior",
    "n/a",
    "### Actual Behavior",
    "",
    "### Steps to Reproduce / How to Trigger",
    "1. Scale the matching service.",
    "### Environment",
    "Prod cluster.",
  ].join("\n");

  const description = parseSections(body, bugRequired).get(
    normalize("Description")
  );
  assert.match(description, /Workers stop polling/);
  assert.match(description, /### Suggested fix/);
  assert.match(description, /Heading text stays in the description/);
  assert.match(description, /\*\*Note\*\*/);
  assert.match(description, /Bold text stays in the description/);
  assert.match(description, /\*\*If yes, explain why:\*\*/);
  assert.match(description, /This prompt stays in the description/);

  assert.deepEqual(findMissingSections(body, bugRequired), [
    "Expected Behavior",
    "Actual Behavior",
    "Logs / Screenshots",
  ]);
});

test("inline code with triple backticks does not open a fence", () => {
  const body = [
    "### Description",
    "```inline``` and ```` ```nested``` ````",
    "### Steps to Reproduce / How to Trigger",
    "1. Start the server.",
    "### Expected Behavior",
    "The workflow completes.",
    "### Actual Behavior",
    "The workflow hangs.",
    "### Logs / Screenshots",
    "~~~",
    "### Environment",
    "~~~",
    "### Environment",
    "Prod cluster.",
  ].join("\n");

  const sections = parseSections(body, bugRequired);
  assert.match(sections.get(normalize("Description")), /```inline```/);
  assert.match(sections.get(normalize("Logs / Screenshots")), /### Environment/);
  assert.equal(sections.get(normalize("Environment")), "Prod cluster.");
  assert.deepEqual(findMissingSections(body, bugRequired), []);
});
