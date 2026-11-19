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
  it("when the body has markdown ### headings, then each heading's text becomes a section keyed by its normalized title", () => {
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
    assert.ok(
      sections.get("steps to reproduce how to trigger").includes("1. Do thing"),
      "steps section should contain its body text",
    );
  });

  it("when the body is null, then no sections are returned", () => {
    assert.equal(parseSections(null).size, 0);
  });

  it("when the body uses **bold** lines as headers, then they are parsed as sections", () => {
    const body = "**Description**\nSome content";
    const sections = parseSections(body);
    assert.equal(sections.get("description"), "Some content");
  });
});

describe("hasMeaningfulContent", () => {
  it("when the content is null, empty, or whitespace, then it is not meaningful", () => {
    assert.equal(hasMeaningfulContent(null), false);
    assert.equal(hasMeaningfulContent(""), false);
    assert.equal(hasMeaningfulContent("   "), false);
  });

  it("when the content is a placeholder (N/A, TBD, none, TODO), then it is not meaningful", () => {
    assert.equal(hasMeaningfulContent("N/A"), false);
    assert.equal(hasMeaningfulContent("TBD"), false);
    assert.equal(hasMeaningfulContent("none"), false);
    assert.equal(hasMeaningfulContent("TODO"), false);
  });

  it("when the content is real text, then it is meaningful", () => {
    assert.equal(hasMeaningfulContent("The server crashes on startup"), true);
  });

  it("when the content is only an HTML comment, then it is not meaningful", () => {
    assert.equal(hasMeaningfulContent("<!-- placeholder -->"), false);
  });
});

describe("detectTemplate", () => {
  it("when the body contains the bug_report marker, then the template is bug_report", () => {
    const body = "<!-- template: bug_report -->\n### Description\nA bug";
    assert.equal(detectTemplate(body), "bug_report");
  });

  it("when the body contains the feature_request marker, then the template is feature_request", () => {
    const body = "<!-- template: feature_request -->\n### Description\nA feature";
    assert.equal(detectTemplate(body), "feature_request");
  });

  it("when the body has no marker or is null, then no template is detected", () => {
    assert.equal(detectTemplate("Just a free-form issue"), null);
    assert.equal(detectTemplate(null), null);
  });
});

describe("detectIssueType", () => {
  it("when the body contains bug template keywords, then the type is kind/bug", () => {
    assert.equal(
      detectIssueType("### Steps to Reproduce\n1. Do thing"),
      "kind/bug",
    );
  });

  it("when the body contains feature template keywords, then the type is kind/feature", () => {
    assert.equal(
      detectIssueType("### Is this a breaking change?\nNo"),
      "kind/feature",
    );
  });

  it("when the body has no template keywords, then no type is detected", () => {
    assert.equal(detectIssueType("Please add dark mode"), null);
  });
});

describe("evaluateIssue", () => {
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

  it("when a bug template is complete and triage/needs-info is already applied, then triage/needs-info is removed and the bot comment is deleted", () => {
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

  it("when a bug template has empty required sections, then triage/needs-info is added and the comment lists the missing sections", () => {
    const result = evaluateIssue({
      title: "Bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, true);
    assert.ok(result.missingSections.length > 0, "should report at least one missing section");
    assert.ok(
      result.comment.includes("issue-template-check"),
      "comment should carry the issue-template-check marker",
    );
    assert.ok(
      result.comment.includes("Missing required sections"),
      "comment should list missing required sections",
    );
  });

  it("when a feature template is complete, then triage/needs-info is not added and no sections are missing", () => {
    const result = evaluateIssue({
      title: "Rate limiting",
      body: completeFeatureBody,
      labels: [{ name: "kind/feature" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.missingSections.length, 0);
  });

  it("when the issue has no template marker, then triage/needs-info is never added and no comment is posted", () => {
    const result = evaluateIssue({
      title: "Please add dark mode",
      body: "I would like dark mode support for the web UI",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.comment, null);
  });

  it("when the issue has no template marker and the body contains bug keywords, then kind/bug is applied and triage/needs-info is not added", () => {
    const result = evaluateIssue({
      title: "Crash on startup",
      body: "### Steps to Reproduce\n1. Start the server\n### Expected Behavior\nIt works\n### Actual Behavior\nCrash",
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, "kind/bug");
    assert.equal(result.addNeedsInfo, false);
  });

  it("when a template is incomplete and a manual triage/* label is present, then triage/needs-info is not added and no comment is posted", () => {
    const result = evaluateIssue({
      title: "Bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }, { name: "triage/needs-decision" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.comment, null);
  });

  it("when a template is incomplete and the author is a MEMBER, then triage/needs-info is not added and no comment is posted", () => {
    const result = evaluateIssue({
      title: "Internal bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "MEMBER",
    });
    assert.equal(result.addNeedsInfo, false);
    assert.equal(result.comment, null);
  });

  it("when a template is incomplete and the author is an OWNER, then triage/needs-info is not added", () => {
    const result = evaluateIssue({
      title: "Owner bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "OWNER",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("when a template is incomplete and the author is a COLLABORATOR, then triage/needs-info is not added", () => {
    const result = evaluateIssue({
      title: "Collaborator bug",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }],
      authorAssociation: "COLLABORATOR",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("when a template is incomplete and the issue has the epic label, then triage/needs-info is not added", () => {
    const result = evaluateIssue({
      title: "Epic tracker",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }, { name: "epic" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("when a template is incomplete and the issue has the roadmap label, then triage/needs-info is not added", () => {
    const result = evaluateIssue({
      title: "Roadmap item",
      body: incompleteBugBody,
      labels: [{ name: "kind/bug" }, { name: "roadmap" }],
      authorAssociation: "NONE",
    });
    assert.equal(result.addNeedsInfo, false);
  });

  it("when the bug_report marker is present and no kind/* label is applied, then kind/bug is applied", () => {
    const result = evaluateIssue({
      title: "A bug",
      body: completeBugBody,
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, "kind/bug");
  });

  it("when the feature_request marker is present and no kind/* label is applied, then kind/feature is applied", () => {
    const result = evaluateIssue({
      title: "A feature",
      body: completeFeatureBody,
      labels: [],
      authorAssociation: "NONE",
    });
    assert.equal(result.typeToApply, "kind/feature");
  });

  it("when the issue has no template marker and no type keywords, then no type is applied, triage/needs-info is not added, and no comment is posted", () => {
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
