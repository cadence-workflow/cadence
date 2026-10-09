const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const path = require("path");

const {
  normalize,
  detectIssueType,
  readTemplateHeadings,
  parseSections,
  hasMeaningfulContent,
  FALLBACK_BUG_REQUIRED,
  FALLBACK_ENHANCEMENT_REQUIRED,
} = require("./validate-issue.js");

const BUG_REQUIRED = FALLBACK_BUG_REQUIRED;

describe("normalize", () => {
  const tests = [
    { input: "Description", expected: "description" },
    {
      input: "Steps to Reproduce / How to Trigger",
      expected: "steps to reproduce how to trigger",
    },
    { input: "  Logs / Screenshots  ", expected: "logs screenshots" },
    {
      input: "Is this a breaking change?",
      expected: "is this a breaking change",
    },
    { input: "ALL CAPS HEADING", expected: "all caps heading" },
    { input: "  extra   spaces  ", expected: "extra spaces" },
  ];

  for (const { input, expected } of tests) {
    it(`normalizes "${input}" to "${expected}"`, () => {
      assert.equal(normalize(input), expected);
    });
  }
});

describe("detectIssueType", () => {
  const tests = [
    {
      name: "detects bug from 'steps to reproduce'",
      body: "### Steps to Reproduce\nDo something",
      expected: "kind/bug",
    },
    {
      name: "detects bug from 'expected behavior'",
      body: "### Expected Behavior\nIt should work",
      expected: "kind/bug",
    },
    {
      name: "detects bug from 'actual behavior'",
      body: "### Actual Behavior\nIt crashed",
      expected: "kind/bug",
    },
    {
      name: "detects feature from 'breaking change'",
      body: "### Is this a breaking change?\nNo",
      expected: "kind/feature",
    },
    {
      name: "detects feature from 'scope of the feature'",
      body: "### Scope of the feature\nServer only",
      expected: "kind/feature",
    },
    {
      name: "returns null for unrecognized body",
      body: "Just some random text",
      expected: null,
    },
    { name: "returns null for null body", body: null, expected: null },
    { name: "returns null for empty body", body: "", expected: null },
  ];

  for (const { name, body, expected } of tests) {
    it(name, () => {
      assert.equal(detectIssueType(body), expected);
    });
  }
});

describe("readTemplateHeadings", () => {
  const workspacePath = path.resolve(__dirname, "../..");

  it("reads headings from bug_report.md", () => {
    const headings = readTemplateHeadings("bug_report.md", workspacePath);
    assert.ok(headings, "should return headings");
    assert.ok(headings.length > 0, "should have at least one heading");
    assert.ok(
      headings.includes("Description"),
      "should include 'Description'"
    );
    assert.ok(
      headings.includes("Environment"),
      "should include 'Environment'"
    );
  });

  it("reads headings from feature_request.md", () => {
    const headings = readTemplateHeadings("feature_request.md", workspacePath);
    assert.ok(headings, "should return headings");
    assert.ok(
      headings.includes("Description"),
      "should include 'Description'"
    );
  });

  it("returns null for nonexistent template", () => {
    const headings = readTemplateHeadings("nonexistent.md", workspacePath);
    assert.equal(headings, null);
  });
});

describe("hasMeaningfulContent", () => {
  const tests = [
    { input: null, expected: false, name: "null" },
    { input: "", expected: false, name: "empty string" },
    { input: "   ", expected: false, name: "whitespace only" },
    { input: "<!-- comment -->", expected: false, name: "HTML comment only" },
    { input: "N/A", expected: false, name: "'N/A'" },
    { input: "n/a", expected: false, name: "'n/a'" },
    { input: "na", expected: false, name: "'na'" },
    { input: "none", expected: false, name: "'none'" },
    { input: "TBD", expected: false, name: "'TBD'" },
    { input: "todo", expected: false, name: "'todo'" },
    { input: "  N/A  ", expected: false, name: "'N/A' with whitespace" },
    { input: "- - -", expected: false, name: "dashes and spaces" },
    { input: "Real content here", expected: true, name: "real content" },
    { input: "- item 1\n- item 2", expected: true, name: "list items" },
    {
      input: "<!-- comment -->\nActual text",
      expected: true,
      name: "comment + real text",
    },
    { input: "```\ncode\n```", expected: true, name: "code block" },
  ];

  for (const { input, expected, name } of tests) {
    it(`returns ${expected} for ${name}`, () => {
      assert.equal(hasMeaningfulContent(input), expected);
    });
  }
});

describe("parseSections", () => {
  it("parses basic headings", () => {
    const body = [
      "### Description",
      "Bug description here",
      "",
      "### Environment",
      "- **Version**: v1.4.2",
    ].join("\n");

    const sections = parseSections(body, BUG_REQUIRED);
    assert.equal(
      sections.get(normalize("Description")),
      "Bug description here"
    );
    assert.equal(
      sections.get(normalize("Environment")),
      "- **Version**: v1.4.2"
    );
  });

  it("parses all six bug template sections", () => {
    const body = [
      "### Description",
      "A bug",
      "### Steps to Reproduce / How to Trigger",
      "1. Do thing",
      "### Expected Behavior",
      "It works",
      "### Actual Behavior",
      "It breaks",
      "### Logs / Screenshots",
      "See logs",
      "### Environment",
      "Prod",
    ].join("\n");

    const sections = parseSections(body, BUG_REQUIRED);
    assert.equal(sections.size, 6);
    for (const heading of BUG_REQUIRED) {
      assert.ok(
        sections.has(normalize(heading)),
        `should have section "${heading}"`
      );
      assert.ok(
        sections.get(normalize(heading)).length > 0,
        `section "${heading}" should have content`
      );
    }
  });

  describe("content lines ending with a colon", () => {
    it("does not split on content lines ending with colon", () => {
      const body = [
        "### Actual Behavior",
        "The server logs this warning periodically:",
        "```",
        "WARN  [history] activity timeout exceeded",
        "```",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const content = sections.get(normalize("Actual Behavior"));
      assert.ok(content, "Actual Behavior should have content");
      assert.ok(
        content.includes("warning periodically:"),
        "should include the colon line"
      );
      assert.ok(
        content.includes("WARN"),
        "should include the code block content"
      );
    });

    it("does not split on 'Follow these steps to reproduce:'", () => {
      const body = [
        "### Steps to Reproduce / How to Trigger",
        "Follow these steps to reproduce:",
        "1. Start the server",
        "2. Register a domain",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const content = sections.get(
        normalize("Steps to Reproduce / How to Trigger")
      );
      assert.ok(content, "Steps section should have content");
      assert.ok(
        content.includes("Follow these steps"),
        "should include intro line"
      );
      assert.ok(
        content.includes("Register a domain"),
        "should include steps"
      );
    });
  });

  describe("bold text inside sections", () => {
    it("does not split on bold text inside a section", () => {
      const body = [
        "### Description",
        "**Important note**",
        "This is a critical regression.",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const content = sections.get(normalize("Description"));
      assert.ok(content, "Description should have content");
      assert.ok(
        content.includes("Important note"),
        "should include bold text"
      );
      assert.ok(
        content.includes("critical regression"),
        "should include following text"
      );
    });

    it("does not split on bold key-value lines", () => {
      const body = [
        "### Environment",
        "- **Cadence server version**: v1.4.2",
        "- **DB & version**: Cassandra 4.0",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const content = sections.get(normalize("Environment"));
      assert.ok(content, "Environment should have content");
      assert.ok(content.includes("v1.4.2"), "should include version");
      assert.ok(
        content.includes("Cassandra"),
        "should include DB info"
      );
    });
  });

  describe("code block handling (#8622)", () => {
    it("ignores YAML keys inside fenced code blocks", () => {
      const body = [
        "### Description",
        "Server fails with this config:",
        "```yaml",
        "persistence:",
        "  defaultStore: cass-default",
        "  datastores:",
        "    cass-default:",
        "      cassandra:",
        "        hosts: 127.0.0.1",
        "dynamicconfig:",
        "  client: filebased",
        "```",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const content = sections.get(normalize("Description"));
      assert.ok(content, "Description should have content");
      assert.ok(
        content.includes("persistence:"),
        "should include YAML as content"
      );
      assert.ok(
        content.includes("dynamicconfig:"),
        "should include dynamicconfig as content"
      );
    });

    it("ignores bash shebangs and comments inside code blocks", () => {
      const body = [
        "### Steps to Reproduce / How to Trigger",
        "Run the script:",
        "```bash",
        "#!/bin/bash",
        "# restart the service",
        "cadence-server start",
        "```",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const content = sections.get(
        normalize("Steps to Reproduce / How to Trigger")
      );
      assert.ok(content, "Steps section should have content");
      assert.ok(
        content.includes("#!/bin/bash"),
        "should include shebang as content"
      );
      assert.ok(
        content.includes("# restart"),
        "should include comment as content"
      );
    });

    it("ignores markdown headings inside code blocks", () => {
      const body = [
        "### Description",
        "Example markdown:",
        "```markdown",
        "### This is not a real heading",
        "## Neither is this",
        "```",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const content = sections.get(normalize("Description"));
      assert.ok(content, "Description should have content");
      assert.ok(
        content.includes("not a real heading"),
        "should include code block heading as content"
      );
    });

    it("resumes heading detection after code block closes", () => {
      const body = [
        "### Description",
        "Some config:",
        "```yaml",
        "persistence:",
        "  driver: cassandra",
        "```",
        "",
        "### Environment",
        "- **Version**: v1.4.2",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      assert.ok(
        sections.has(normalize("Description")),
        "should have Description"
      );
      assert.ok(
        sections.has(normalize("Environment")),
        "should have Environment"
      );
      const desc = sections.get(normalize("Description"));
      assert.ok(
        desc.includes("persistence:"),
        "Description should include code block"
      );
    });

    it("keeps shorter backtick runs inside longer fenced blocks", () => {
      const body = [
        "### Description",
        "Example containing a nested code fence:",
        "````markdown",
        "```",
        "### This is still code",
        "````",
        "### Environment",
        "Prod",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const description = sections.get(normalize("Description"));
      assert.ok(description.includes("### This is still code"));
      assert.equal(sections.get(normalize("Environment")), "Prod");
    });
  });

  describe("non-required headings", () => {
    it("does not split on headings that are not in required sections", () => {
      const body = [
        "### Description",
        "The bug.",
        "### Suggested fix",
        "Remove the regex.",
        "### Environment",
        "Prod",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const desc = sections.get(normalize("Description"));
      assert.ok(desc, "Description should have content");
      assert.ok(
        desc.includes("The bug."),
        "should include first line"
      );
      assert.ok(
        desc.includes("Suggested fix"),
        "should include non-required heading as content"
      );
      assert.ok(
        desc.includes("Remove the regex"),
        "should include text under non-required heading"
      );
    });
  });

  describe("edge cases", () => {
    it("returns empty map for null body", () => {
      const sections = parseSections(null, BUG_REQUIRED);
      assert.equal(sections.size, 0);
    });

    it("returns empty map for empty body", () => {
      const sections = parseSections("", BUG_REQUIRED);
      assert.equal(sections.size, 0);
    });

    it("handles empty sections", () => {
      const body = [
        "### Description",
        "",
        "### Environment",
        "Prod",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      assert.equal(sections.get(normalize("Description")), "");
    });

    it("handles section with only HTML comment", () => {
      const body = [
        "### Description",
        "<!-- placeholder -->",
        "### Environment",
        "Prod",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const desc = sections.get(normalize("Description"));
      assert.equal(desc, "<!-- placeholder -->");
      assert.equal(
        hasMeaningfulContent(desc),
        false,
        "comment-only content should not be meaningful"
      );
    });

    it("handles Windows-style line endings", () => {
      const body =
        "### Description\r\nBug here\r\n### Environment\r\nProd\r\n";

      const sections = parseSections(body, BUG_REQUIRED);
      assert.equal(
        sections.get(normalize("Description")),
        "Bug here"
      );
    });

    it("ignores text before the first required heading", () => {
      const body = [
        "Some preamble text",
        "More preamble",
        "### Description",
        "The real content",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const desc = sections.get(normalize("Description"));
      assert.equal(desc, "The real content");
      assert.ok(
        !Array.from(sections.values()).some((v) =>
          v.includes("preamble")
        ),
        "preamble should not appear in any section"
      );
    });
  });

  describe("issue #8621 regression pattern", () => {
    it("correctly parses a bug report where Actual Behavior starts with a colon line", () => {
      const body = [
        "<!-- template: bug_report -->",
        "",
        "### Description",
        "The workflow execution gets stuck after activity completion.",
        "",
        "### Steps to Reproduce / How to Trigger",
        "1. Start a workflow with a long-running activity",
        "2. Wait for the activity to complete",
        "3. Observe the workflow does not proceed",
        "",
        "### Expected Behavior",
        "The workflow should continue to the next step after activity completion.",
        "",
        "### Actual Behavior",
        "The server logs this warning periodically:",
        "```",
        "WARN  [history] activity timeout exceeded for workflowID=test-workflow",
        "ERROR [matching] task list backlog growing: tasklist=test-tasklist",
        "```",
        "The workflow remains stuck indefinitely.",
        "",
        "### Logs / Screenshots",
        "See the logs above.",
        "",
        "### Environment",
        "- **Cadence server version**: v1.4.2",
        "- **Cadence SDK language and version**: Go SDK v1.2.0",
        "- **DB & version**: Cassandra 4.0",
        "- **Scale**: Single node dev cluster",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const missingSections = BUG_REQUIRED.filter(
        (s) => !hasMeaningfulContent(sections.get(normalize(s)))
      );

      assert.deepEqual(
        missingSections,
        [],
        `should have no missing sections but got: [${missingSections.join(", ")}]`
      );

      const actual = sections.get(normalize("Actual Behavior"));
      assert.ok(
        actual.includes("warning periodically:"),
        "should keep colon line"
      );
      assert.ok(
        actual.includes("remains stuck"),
        "should keep text after code block"
      );
    });
  });

  describe("issue #8620 regression pattern", () => {
    it("correctly parses a bug report where Steps starts with a colon line", () => {
      const body = [
        "<!-- template: bug_report -->",
        "",
        "### Description",
        "Tasks are not being dispatched to workers.",
        "",
        "### Steps to Reproduce / How to Trigger",
        "Follow these steps to reproduce:",
        "1. Start the server with default config",
        "2. Register a domain using cadence-cli",
        "3. Start a workflow that schedules activities",
        "",
        "### Expected Behavior",
        "Activities should be dispatched to workers within the configured SLA.",
        "",
        "### Actual Behavior",
        "No activities are dispatched. Workers remain idle.",
        "",
        "### Logs / Screenshots",
        "```",
        "INFO  [matching] No tasks available for tasklist=default",
        "```",
        "",
        "### Environment",
        "- **Cadence server version**: v1.4.1",
        "- **DB & version**: MySQL 8.0",
        "- **Scale**: 3-node cluster",
      ].join("\n");

      const sections = parseSections(body, BUG_REQUIRED);
      const missingSections = BUG_REQUIRED.filter(
        (s) => !hasMeaningfulContent(sections.get(normalize(s)))
      );

      assert.deepEqual(
        missingSections,
        [],
        `should have no missing sections but got: [${missingSections.join(", ")}]`
      );
    });
  });
});

