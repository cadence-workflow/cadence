"use strict";

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const {
  LABELS,
  parseSections,
  hasMeaningfulContent,
  detectTemplate,
  detectIssueType,
  evaluateIssue,
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

describe("evaluateIssue — deterministic guards", () => {
  const completeBugBody = [
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

  const completeFeatureBody = [
    "<!-- template: feature_request -->",
    "### Description",
    "Add rate limiting",
    "### Is this a breaking change?",
    "No",
    "### Scope of the feature (server, specific client, all clients)",
    "Server only",
  ].join("\n");

  it("complete template bug: no needs-info, removes existing", () => {
    const result = evaluateIssue({
      title: "Server crash",
      body: completeBugBody,
      labels: [{ name: "kind/bug" }, { name: "triage/needs-info" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.removeNeedsInfo, true);
    assert.equal(result.typeToApply, null);
    assert.equal(result.comment, null);
    assert.equal(result.deleteComment, true);
  });

  it("incomplete template bug: adds needs-info with missing sections", () => {
    const result = evaluateIssue({
      title: "Bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, true);
    assert.ok(result.missingSections.length > 0);
    assert.ok(result.comment.includes("issue-template-check"));
    assert.ok(result.comment.includes("Missing required sections"));
  });

  it("complete template feature: no needs-info", () => {
    const result = evaluateIssue({
      title: "Rate limiting",
      body: completeFeatureBody,
      labels: [{ name: "kind/feature" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.missingSections.length, 0);
  });

  it("free-form (no template marker): NEVER adds needs-info", () => {
    const result = evaluateIssue({
      title: "Please add dark mode",
      body: "I would like dark mode support for the web UI",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.comment, null);
  });

  it("free-form with bug keywords: auto-detects type but no needs-info", () => {
    const result = evaluateIssue({
      title: "Crash on startup",
      body: "### Steps to Reproduce\n1. Start the server\n### Expected Behavior\nIt works\n### Actual Behavior\nCrash",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, "kind/bug");
    assert.equal(result.addNeedsInfo, false);
  });

  it("template issue with existing triage/* label: skips needs-info", () => {
    const result = evaluateIssue({
      title: "Bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }, { name: "triage/needs-decision" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.comment, null);
  });

  it("template issue from MEMBER author: skips needs-info", () => {
    const result = evaluateIssue({
      title: "Internal bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "MEMBER",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.comment, null);
  });

  it("template issue from OWNER author: skips needs-info", () => {
    const result = evaluateIssue({
      title: "Owner bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "OWNER",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("template issue from COLLABORATOR author: skips needs-info", () => {
    const result = evaluateIssue({
      title: "Collaborator bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "COLLABORATOR",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("issue with epic label: skips needs-info", () => {
    const result = evaluateIssue({
      title: "Epic tracker",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }, { name: "epic" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("issue with roadmap label: skips needs-info", () => {
    const result = evaluateIssue({
      title: "Roadmap item",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }, { name: "roadmap" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("template bug without kind/* label: auto-applies kind/bug from marker", () => {
    const result = evaluateIssue({
      title: "A bug",
      body: completeBugBody,
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, "kind/bug");
  });

  it("template feature without kind/* label: auto-applies kind/feature from marker", () => {
    const result = evaluateIssue({
      title: "A feature",
      body: completeFeatureBody,
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, "kind/feature");
  });

  it("non-template vague issue: no type, no needs-info", () => {
    const result = evaluateIssue({
      title: "Something",
      body: "A vague issue",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, null);
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.comment, null);
  });
});
