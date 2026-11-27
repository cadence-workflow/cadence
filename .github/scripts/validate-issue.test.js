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
  run,
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

describe("run", () => {
  const MARKER = "<!-- issue-template-check -->";

  const MISSING_BODY = "### Description\nSomething broke";
  const COMPLETE_BODY = [
    "### Description",
    "The server crashes on startup",
    "### Steps to Reproduce / How to Trigger",
    "1. Start the server",
    "### Expected Behavior",
    "It starts",
    "### Actual Behavior",
    "It crashes",
    "### Logs / Screenshots",
    "panic: nil pointer",
    "### Environment",
    "linux/amd64",
  ].join("\n");

  function fakeGitHub({ comments = [], labelExists = true } = {}) {
    const calls = [];
    const rest = {
      issues: {
        listComments: "listComments",
        addLabels: async (args) => calls.push(["addLabels", args]),
        removeLabel: async (args) => calls.push(["removeLabel", args]),
        createComment: async (args) => calls.push(["createComment", args]),
        updateComment: async (args) => calls.push(["updateComment", args]),
        deleteComment: async (args) => calls.push(["deleteComment", args]),
        createLabel: async (args) => calls.push(["createLabel", args]),
        getLabel: async () => {
          if (!labelExists) {
            const error = new Error("not found");
            error.status = 404;
            throw error;
          }
          return {};
        },
      },
    };
    const paginate = async (method) => {
      if (method === "listComments") return comments;
      throw new Error(`unexpected paginate ${method}`);
    };
    return { github: { rest, paginate }, calls };
  }

  function contextFor(body, labels) {
    return {
      payload: {
        issue: {
          number: 7,
          title: "Server crashes",
          body,
          labels: labels.map((name) => ({ name })),
          author_association: "NONE",
        },
      },
      repo: { owner: "cadence-workflow", repo: "cadence" },
    };
  }

  it("refreshes the bot comment when the label already exists", async () => {
    const { github, calls } = fakeGitHub({
      comments: [{ id: 7, user: { type: "Bot" }, body: `${MARKER}\nold` }],
    });

    await run({
      github,
      context: contextFor(MISSING_BODY, ["kind/bug", "triage/needs-info"]),
    });

    assert.deepEqual(
      calls.map(([name]) => name),
      ["updateComment"],
    );
    const body = calls[0][1].body;
    assert.ok(body.includes("Steps to Reproduce / How to Trigger"));
    assert.ok(!body.includes("- Description"));
  });

  it("does not rewrite an up-to-date bot comment", async () => {
    const first = fakeGitHub({
      comments: [{ id: 7, user: { type: "Bot" }, body: `${MARKER}\nold` }],
    });
    await run({
      github: first.github,
      context: contextFor(MISSING_BODY, ["kind/bug", "triage/needs-info"]),
    });
    const updatedBody = first.calls[0][1].body;

    const second = fakeGitHub({
      comments: [{ id: 7, user: { type: "Bot" }, body: updatedBody }],
    });
    await run({
      github: second.github,
      context: contextFor(MISSING_BODY, ["kind/bug", "triage/needs-info"]),
    });

    assert.deepEqual(second.calls, []);
  });

  it("adds the label and posts the comment on the first pass", async () => {
    const { github, calls } = fakeGitHub();

    await run({ github, context: contextFor(MISSING_BODY, ["kind/bug"]) });

    assert.deepEqual(
      calls.map(([name]) => name),
      ["addLabels", "createComment"],
    );
    assert.equal(calls[0][1].labels[0], "triage/needs-info");
  });

  it("creates the label when the repository does not have it yet", async () => {
    const { github, calls } = fakeGitHub({ labelExists: false });

    await run({ github, context: contextFor(MISSING_BODY, ["kind/bug"]) });

    assert.deepEqual(
      calls.map(([name]) => name),
      ["createLabel", "addLabels", "createComment"],
    );
  });

  it("removes the label and deletes the bot comment once the issue is complete", async () => {
    const { github, calls } = fakeGitHub({
      comments: [{ id: 7, user: { type: "Bot" }, body: `${MARKER}\nold` }],
    });

    await run({
      github,
      context: contextFor(COMPLETE_BODY, ["kind/bug", "triage/needs-info"]),
    });

    assert.deepEqual(
      calls.map(([name]) => name),
      ["removeLabel", "deleteComment"],
    );
  });

  it("deletes a stale bot comment when no label is present", async () => {
    const { github, calls } = fakeGitHub({
      comments: [{ id: 7, user: { type: "Bot" }, body: `${MARKER}\nold` }],
    });

    await run({ github, context: contextFor(COMPLETE_BODY, ["kind/bug"]) });

    assert.deepEqual(
      calls.map(([name]) => name),
      ["deleteComment"],
    );
  });

  it("reports needsInfo separately from the label transition", () => {
    const result = evaluateIssue({
      title: "Server crashes",
      body: MISSING_BODY,
      labels: [{ name: "kind/bug" }, { name: "triage/needs-info" }],
      authorAssociation: "NONE",
      templates: null,
    });

    assert.equal(result.needsInfo, true);
    assert.equal(result.addNeedsInfo, false);
  });
});
