"use strict";

const MARKER = "<!-- restrict-workflow-changes -->";
const VALID_PERMISSIONS = ["admin", "maintain", "write"];
const WORKFLOWS_DIR = ".github/workflows/";

function findWorkflowFiles(filenames) {
  return (filenames || []).filter((name) => name.startsWith(WORKFLOWS_DIR));
}

function hasValidPermission(permission) {
  return VALID_PERMISSIONS.includes(permission);
}

function buildComment({ owner, repo, workflowFiles }) {
  return [
    MARKER,
    "⚠️ **Workflow changes require maintainer authorship**",
    "",
    "This PR modifies the following GitHub Actions workflow file(s):",
    ...workflowFiles.map((name) => `- \`${name}\``),
    "",
    "Only repository maintainers can change `.github/workflows/`, because these files control CI execution and have access to repo secrets.",
    "",
    "**Please:**",
    "1. Revert the workflow changes from this PR so the rest can be reviewed.",
    `2. Open a [new issue](https://github.com/${owner}/${repo}/issues/new) that describes the workflow change you want, and a maintainer will make it in a separate PR.`,
  ].join("\n");
}

function findBotComment(comments) {
  return (comments || []).find(
    (c) => c.user?.type === "Bot" && c.body?.includes(MARKER),
  );
}

// Returns the repository permission of the user. It includes access granted
// through teams, for example cadence-dev.
async function getPermission({ github, owner, repo, username }) {
  try {
    const { data } = await github.rest.repos.getCollaboratorPermissionLevel({
      owner,
      repo,
      username,
    });
    return data.permission;
  } catch (error) {
    if (error.status !== 404) throw error;
    return "none";
  }
}

async function run({ github, context, core }) {
  const { owner, repo } = context.repo;
  const pr = context.payload.pull_request;
  const author = pr.user.login;

  const files = await github.paginate(github.rest.pulls.listFiles, {
    owner,
    repo,
    pull_number: pr.number,
    per_page: 100,
  });
  const workflowFiles = findWorkflowFiles(files.map((f) => f.filename));

  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number: pr.number,
    per_page: 100,
  });
  const existing = findBotComment(comments);

  // Remove the warning after the author reverts the workflow changes.
  if (workflowFiles.length === 0) {
    if (existing) {
      await github.rest.issues.deleteComment({
        owner,
        repo,
        comment_id: existing.id,
      });
    }
    return;
  }

  const permission = await getPermission({
    github,
    owner,
    repo,
    username: author,
  });
  if (hasValidPermission(permission)) {
    core.info(`${author} has ${permission} access, workflow changes are allowed`);
    return;
  }

  const body = buildComment({ owner, repo, workflowFiles });
  if (existing) {
    await github.rest.issues.updateComment({
      owner,
      repo,
      comment_id: existing.id,
      body,
    });
  } else {
    await github.rest.issues.createComment({
      owner,
      repo,
      issue_number: pr.number,
      body,
    });
  }

  core.setFailed(
    `${author} has ${permission} access and cannot change workflow files`,
  );
}

module.exports = {
  MARKER,
  VALID_PERMISSIONS,
  findWorkflowFiles,
  hasValidPermission,
  buildComment,
  findBotComment,
  getPermission,
  run,
};
