"use strict";

const { describe, it } = require("node:test");
const assert = require("node:assert/strict");
const {
  MARKER,
  findWorkflowFiles,
  hasValidPermission,
  buildComment,
  findBotComment,
  run,
} = require("./restrict-workflow-changes.js");

describe("findWorkflowFiles", () => {
  it("returns only files under .github/workflows/", () => {
    const files = [
      ".github/workflows/ci-checks.yml",
      ".github/scripts/validate-issue.js",
      "service/history/handler.go",
      ".github/workflows/nested/file.yml",
    ];
    assert.deepEqual(findWorkflowFiles(files), [
      ".github/workflows/ci-checks.yml",
      ".github/workflows/nested/file.yml",
    ]);
  });

  it("returns an empty list for null", () => {
    assert.deepEqual(findWorkflowFiles(null), []);
  });
});

describe("hasValidPermission", () => {
  const tests = [
    { permission: "admin", want: true },
    { permission: "maintain", want: true },
    { permission: "write", want: true },
    { permission: "triage", want: false },
    { permission: "read", want: false },
    { permission: "none", want: false },
  ];
  for (const tt of tests) {
    it(`${tt.permission} is ${tt.want ? "valid" : "not valid"}`, () => {
      assert.equal(hasValidPermission(tt.permission), tt.want);
    });
  }
});

describe("buildComment", () => {
  it("includes the marker, the files and the issue link", () => {
    const body = buildComment({
      owner: "cadence-workflow",
      repo: "cadence",
      workflowFiles: [".github/workflows/ci-checks.yml"],
    });
    assert.ok(body.startsWith(MARKER));
    assert.ok(body.includes("- `.github/workflows/ci-checks.yml`"));
    assert.ok(
      body.includes("https://github.com/cadence-workflow/cadence/issues/new"),
    );
  });
});

describe("findBotComment", () => {
  it("finds the bot comment with the marker", () => {
    const comments = [
      { id: 1, user: { type: "User" }, body: `${MARKER} copied by a user` },
      { id: 2, user: { type: "Bot" }, body: "another bot comment" },
      { id: 3, user: { type: "Bot" }, body: `${MARKER}\nwarning` },
    ];
    assert.equal(findBotComment(comments).id, 3);
  });

  it("returns undefined when there is no comment", () => {
    assert.equal(findBotComment([]), undefined);
  });
});

function fakeGitHub({ files, comments, permission, permissionStatus }) {
  const calls = [];
  const rest = {
    pulls: { listFiles: "listFiles" },
    issues: {
      listComments: "listComments",
      createComment: async (args) => calls.push(["create", args]),
      updateComment: async (args) => calls.push(["update", args]),
      deleteComment: async (args) => calls.push(["delete", args]),
    },
    repos: {
      getCollaboratorPermissionLevel: async () => {
        calls.push(["permission"]);
        if (permissionStatus) {
          const error = new Error("request failed");
          error.status = permissionStatus;
          throw error;
        }
        return { data: { permission } };
      },
    },
  };
  const paginate = async (method) => {
    if (method === "listFiles") return files.map((filename) => ({ filename }));
    if (method === "listComments") return comments;
    throw new Error(`unexpected method ${method}`);
  };
  return { github: { rest, paginate }, calls };
}

function fakeCore() {
  return { failed: null, info: () => {}, setFailed(msg) { this.failed = msg; } };
}

const context = {
  repo: { owner: "cadence-workflow", repo: "cadence" },
  payload: { pull_request: { number: 9, user: { login: "contributor" } } },
};

const botComment = { id: 7, user: { type: "Bot" }, body: `${MARKER}\nwarning` };

describe("run", () => {
  const tests = [
    {
      name: "no workflow files and no comment does nothing",
      input: { files: ["service/a.go"], comments: [] },
      wantCalls: [],
      wantFailed: false,
    },
    {
      name: "no workflow files deletes the old comment",
      input: { files: ["service/a.go"], comments: [botComment] },
      wantCalls: ["delete"],
      wantFailed: false,
    },
    {
      name: "author with write access passes",
      input: {
        files: [".github/workflows/ci.yml"],
        comments: [],
        permission: "write",
      },
      wantCalls: ["permission"],
      wantFailed: false,
    },
    {
      name: "author with read access gets a comment and fails",
      input: {
        files: [".github/workflows/ci.yml"],
        comments: [],
        permission: "read",
      },
      wantCalls: ["permission", "create"],
      wantFailed: true,
    },
    {
      name: "existing comment is updated",
      input: {
        files: [".github/workflows/ci.yml"],
        comments: [botComment],
        permission: "read",
      },
      wantCalls: ["permission", "update"],
      wantFailed: true,
    },
    {
      name: "author who is not a collaborator fails",
      input: {
        files: [".github/workflows/ci.yml"],
        comments: [],
        permissionStatus: 404,
      },
      wantCalls: ["permission", "create"],
      wantFailed: true,
    },
  ];
  for (const tt of tests) {
    it(tt.name, async () => {
      const { github, calls } = fakeGitHub(tt.input);
      const core = fakeCore();
      await run({ github, context, core });
      assert.deepEqual(
        calls.map((c) => c[0]),
        tt.wantCalls,
      );
      assert.equal(core.failed !== null, tt.wantFailed);
    });
  }

  it("throws on unexpected permission errors", async () => {
    const { github } = fakeGitHub({
      files: [".github/workflows/ci.yml"],
      comments: [],
      permissionStatus: 500,
    });
    await assert.rejects(run({ github, context, core: fakeCore() }));
  });
});
