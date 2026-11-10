"use strict";

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const {
  LABELS,
  normalize,
  parseSections,
  findMissingSections,
  hasMeaningfulContent,
  detectTemplate,
  detectIssueType,
  evaluateIssue,
  readTemplateHeadings,
} = require("./validate-issue.js");

describe("parseSections", () => {
  it("parses markdown headings into sections", () => {
    const body = [
      "### Description",
      "My bug description",
      "",
      "### Steps to Reproduce / How to Trigger",
      "1. Do thing",
      "2. See error",
    ].join("\n");

    const sections = parseSections(body);
    assert.equal(sections.get("description"), "My bug description");
    assert.ok(sections.get("steps to reproduce how to trigger").includes("1. Do thing"));
  });

  it("returns empty map for null body", () => {
    assert.equal(parseSections(null).size, 0);
  });

  it("parses bold-style section headers", () => {
    const body = "**Description**\nSome content";
    const sections = parseSections(body);
    assert.equal(sections.get("description"), "Some content");
  });
});

describe("hasMeaningfulContent", () => {
  it("returns false for empty/null", () => {
    assert.equal(hasMeaningfulContent(null), false);
    assert.equal(hasMeaningfulContent(""), false);
    assert.equal(hasMeaningfulContent("   "), false);
  });

  it("returns false for placeholder values", () => {
    assert.equal(hasMeaningfulContent("N/A"), false);
    assert.equal(hasMeaningfulContent("TBD"), false);
    assert.equal(hasMeaningfulContent("none"), false);
    assert.equal(hasMeaningfulContent("TODO"), false);
  });

  it("returns true for real content", () => {
    assert.equal(hasMeaningfulContent("The server crashes on startup"), true);
  });

  it("ignores HTML comments", () => {
    assert.equal(hasMeaningfulContent("<!-- placeholder -->"), false);
  });
});

describe("detectTemplate", () => {
  it("detects bug_report template marker", () => {
    const body = "<!-- template: bug_report -->\n### Description\nA bug";
    assert.equal(detectTemplate(body), "bug_report");
  });

  it("detects feature_request template marker", () => {
    const body = "<!-- template: feature_request -->\n### Description\nA feature";
    assert.equal(detectTemplate(body), "feature_request");
  });

  it("returns null for no marker", () => {
    assert.equal(detectTemplate("Just a free-form issue"), null);
    assert.equal(detectTemplate(null), null);
  });
});

describe("detectIssueType", () => {
  it("detects bug from body keywords", () => {
    assert.equal(
      detectIssueType("### Steps to Reproduce\n1. Do thing"),
      "kind/bug",
    );
  });

  it("detects feature from body keywords", () => {
    assert.equal(
      detectIssueType("### Is this a breaking change?\nNo"),
      "kind/feature",
    );
  });

  it("returns null for non-template body", () => {
    assert.equal(detectIssueType("Please add dark mode"), null);
  });
});

describe("evaluateIssue", () => {
  const bugBody = [
    "<!-- template: bug_report -->",
    "### Description",
    "Server crashes",
    "### Steps to Reproduce / How to Trigger",
    "1. Start server",
    "### Expected Behavior",
    "Server starts",
    "### Actual Behavior",
    "Server crashes",
    "### Logs / Screenshots",
    "stack trace here",
    "### Environment",
    "Linux, v1.0",
  ].join("\n");

  const incompleteBugBody = [
    "<!-- template: bug_report -->",
    "### Description",
    "Something broke",
    "### Steps to Reproduce / How to Trigger",
    "N/A",
    "### Expected Behavior",
    "",
    "### Actual Behavior",
    "",
    "### Logs / Screenshots",
    "",
    "### Environment",
    "",
  ].join("\n");

  const featureBody = [
    "<!-- template: feature_request -->",
    "### Description",
    "Add rate limiting",
    "### Is this a breaking change?",
    "No",
    "### Scope of the feature (server, specific client, all clients)",
    "Server only",
  ].join("\n");

  it("complete bug: no needs-info, removes existing", () => {
    const result = evaluateIssue({
      title: "Server crash",
      body: bugBody,
      labels: [{ name: "kind/bug" }, { name: "triage/needs-info" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.removeNeedsInfo, true);
    assert.equal(result.typeToApply, null);
    assert.equal(result.comment, null);
    assert.equal(result.deleteComment, true);
  });

  it("incomplete bug: adds needs-info with missing sections", () => {
    const result = evaluateIssue({
      title: "Bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, true);
    assert.ok(result.missingSections.length > 0);
    assert.ok(result.comment.includes("issue-template-check"));
  });

  it("complete feature: no needs-info", () => {
    const result = evaluateIssue({
      title: "Rate limiting",
      body: featureBody,
      labels: [{ name: "kind/feature" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.missingSections.length, 0);
  });

  it("free-form non-template: auto-detects type if keywords present", () => {
    const result = evaluateIssue({
      title: "Please add dark mode",
      body: "I would like dark mode support for the web UI",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, true);
    assert.ok(result.comment.includes("Add one label from"));
  });

  it("no type detected and no label: requests label", () => {
    const result = evaluateIssue({
      title: "Something",
      body: "A vague issue",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, null);
    assert.equal(result.addNeedsInfo, true);
    assert.ok(result.comment.includes("Add one label from"));
  });

  it("already has triage/* label: does not double-add needs-info", () => {
    const result = evaluateIssue({
      title: "Bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }, { name: "triage/needs-info" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.ok(result.missingSections.length > 0);
  });

  it("colon-ending prose does not split a required section", () => {
    const result = evaluateIssue({
      title: "Matching stalls",
      body: bugBody.replace(
        "Server crashes\n### Logs",
        "Server crashes\nThe server logs this warning periodically:\nretrying task list\n### Logs",
      ),
      labels: [{ name: "kind/bug" }],
      authorAssociation: "NONE",
    });
    assert.deepEqual(result.missingSections, []);
    assert.equal(result.addNeedsInfo, false);
  });

  it("auto-applies type label when missing but detected from body", () => {
    const result = evaluateIssue({
      title: "Crash on startup",
      body: "### Steps to Reproduce\n1. Start the server\n### Expected Behavior\nIt works\n### Actual Behavior\nCrash",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, "kind/bug");
  });
});

describe("template-aware parsing", () => {
  const bugRequired = readTemplateHeadings("bug_report.md");
  const featureRequired = readTemplateHeadings("feature_request.md");

  it("required headings are read from the issue templates", () => {
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

  it("colon-ending prose stays in the required section", () => {
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

  it("fenced yaml, shell, and markdown lines stay in the section", () => {
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

  it("required template headings flush the previous section", () => {
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

  it("non-template headings and bold lines stay content while empty sections stay missing", () => {
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

  it("inline code with triple backticks does not open a fence", () => {
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
});
