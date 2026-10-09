const fs = require("fs");
const path = require("path");

const TYPE_LABELS = ["kind/bug", "kind/feature", "kind/cleanup"];
const BUG_LABEL = "kind/bug";
const ENHANCEMENT_LABELS = ["kind/feature", "kind/cleanup"];
const NEEDS_INFO_LABEL = "triage/needs-info";

const FALLBACK_BUG_REQUIRED = [
  "Description",
  "Steps to Reproduce / How to Trigger",
  "Expected Behavior",
  "Actual Behavior",
  "Logs / Screenshots",
  "Environment",
];

const FALLBACK_ENHANCEMENT_REQUIRED = [
  "Description",
  "Is this a breaking change?",
  "Scope of the feature (server, specific client, all clients)",
];

const normalize = (title) =>
  title
    .toLowerCase()
    .replace(/[^\w\s]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();

const detectIssueType = (body) => {
  if (!body) return null;
  const normalizedBody = body.toLowerCase();

  if (
    normalizedBody.includes("steps to reproduce") ||
    normalizedBody.includes("expected behavior") ||
    normalizedBody.includes("actual behavior")
  ) {
    return "kind/bug";
  }

  if (
    normalizedBody.includes("breaking change") ||
    normalizedBody.includes("scope of the feature")
  ) {
    return "kind/feature";
  }

  return null;
};

const readTemplateHeadings = (fileName, workspacePath) => {
  try {
    const templatePath = path.join(
      workspacePath,
      ".github",
      "ISSUE_TEMPLATE",
      fileName
    );
    const contents = fs.readFileSync(templatePath, "utf8");
    const headings = [];
    for (const line of contents.split(/\r?\n/)) {
      const match = line.match(/^###\s+(.+?)\s*$/);
      if (match?.[1]) {
        headings.push(match[1].trim());
      }
    }
    return headings.length > 0 ? headings : null;
  } catch {
    return null;
  }
};

const parseSections = (body, requiredSections) => {
  const sections = new Map();
  if (!body) {
    return sections;
  }
  const requiredSectionKeys = new Set(requiredSections.map(normalize));
  const lines = body.split(/\r?\n/);
  let currentKey = null;
  let buffer = [];
  let inCodeBlock = false;
  let codeFenceLength = 0;

  const flush = () => {
    if (currentKey) {
      sections.set(currentKey, buffer.join("\n").trim());
    }
    buffer = [];
  };

  for (const line of lines) {
    const trimmed = line.trim();
    const fenceMatch = trimmed.match(/^(`{3,})/);
    if (fenceMatch) {
      const fenceLength = fenceMatch[1].length;
      const isClosingFence =
        inCodeBlock &&
        fenceLength >= codeFenceLength &&
        /^`+\s*$/.test(trimmed);

      if (!inCodeBlock) {
        inCodeBlock = true;
        codeFenceLength = fenceLength;
      } else if (isClosingFence) {
        inCodeBlock = false;
        codeFenceLength = 0;
      }
      if (currentKey) {
        buffer.push(line);
      }
      continue;
    }

    const headingMatch = inCodeBlock
      ? null
      : trimmed.match(/^#{1,6}\s+(.+?)\s*$/);
    const title = headingMatch?.[1];
    const key = title ? normalize(title) : null;

    if (key && requiredSectionKeys.has(key)) {
      flush();
      currentKey = key;
      continue;
    }
    if (currentKey) {
      buffer.push(line);
    }
  }

  flush();
  return sections;
};

const hasMeaningfulContent = (content) => {
  if (!content) {
    return false;
  }
  const cleaned = content
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/[\s*-]+/g, " ")
    .trim()
    .toLowerCase();
  if (!cleaned) {
    return false;
  }
  return !["n/a", "na", "none", "tbd", "todo"].includes(cleaned);
};

async function run(github, context) {
  const issue = context.payload.issue;
  const issueNumber = issue.number;
  const owner = context.repo.owner;
  const repo = context.repo.repo;

  const labelNames = (issue.labels || [])
    .map((label) => (typeof label === "string" ? label : label.name))
    .filter(Boolean)
    .map((label) => label.toLowerCase());

  let hasTypeLabel = TYPE_LABELS.some((label) => labelNames.includes(label));
  let isBug = labelNames.includes(BUG_LABEL);
  let isEnhancement = ENHANCEMENT_LABELS.some((label) =>
    labelNames.includes(label)
  );
  let missingLabel = !hasTypeLabel;

  const detectedType = detectIssueType(issue.body);

  if (missingLabel && detectedType) {
    await github.rest.issues.addLabels({
      owner,
      repo,
      issue_number: issueNumber,
      labels: [detectedType],
    });
    labelNames.push(detectedType);
    hasTypeLabel = TYPE_LABELS.some((label) => labelNames.includes(label));
    isBug = labelNames.includes(BUG_LABEL);
    isEnhancement = ENHANCEMENT_LABELS.some((label) =>
      labelNames.includes(label)
    );
    missingLabel = !hasTypeLabel;
  }

  const workspacePath = process.env.GITHUB_WORKSPACE || process.cwd();
  const BUG_REQUIRED =
    readTemplateHeadings("bug_report.md", workspacePath) ||
    FALLBACK_BUG_REQUIRED;
  const ENHANCEMENT_REQUIRED =
    readTemplateHeadings("feature_request.md", workspacePath) ||
    FALLBACK_ENHANCEMENT_REQUIRED;

  const requiredSections = isBug
    ? BUG_REQUIRED
    : isEnhancement
      ? ENHANCEMENT_REQUIRED
      : [];

  const sections = parseSections(issue.body || "", requiredSections);
  const missingSections = requiredSections.filter((section) => {
    const content = sections.get(normalize(section));
    return !hasMeaningfulContent(content);
  });

  const ensureNeedsInfoLabelExists = async () => {
    try {
      await github.rest.issues.getLabel({
        owner,
        repo,
        name: NEEDS_INFO_LABEL,
      });
    } catch (error) {
      if (error.status !== 404) {
        throw error;
      }
      await github.rest.issues.createLabel({
        owner,
        repo,
        name: NEEDS_INFO_LABEL,
        color: "d876e3",
        description:
          "Blocked pending more info from the reporter (repro, version, logs) before triage.",
      });
    }
  };

  const findExistingBotComment = async () => {
    const comments = await github.paginate(
      github.rest.issues.listComments,
      { owner, repo, issue_number: issueNumber, per_page: 100 }
    );
    return comments.find(
      (comment) =>
        comment.user?.type === "Bot" &&
        comment.body?.includes("<!-- issue-template-check -->")
    );
  };

  const updateOrCreateComment = async (body) => {
    const existing = await findExistingBotComment();
    if (existing) {
      await github.rest.issues.updateComment({
        owner,
        repo,
        comment_id: existing.id,
        body,
      });
      return;
    }
    await github.rest.issues.createComment({
      owner,
      repo,
      issue_number: issueNumber,
      body,
    });
  };

  const deleteExistingComment = async () => {
    const existing = await findExistingBotComment();
    if (!existing) {
      return;
    }
    await github.rest.issues.deleteComment({
      owner,
      repo,
      comment_id: existing.id,
    });
  };

  if (missingLabel || missingSections.length > 0) {
    await ensureNeedsInfoLabelExists();
    if (!labelNames.includes(NEEDS_INFO_LABEL)) {
      await github.rest.issues.addLabels({
        owner,
        repo,
        issue_number: issueNumber,
        labels: [NEEDS_INFO_LABEL],
      });
    }

    const templateLines = [
      "Issue templates:",
      `- Bug report: https://github.com/${owner}/${repo}/issues/new?template=bug_report.md`,
      `- Feature request (use for features, improvements, or cleanup): https://github.com/${owner}/${repo}/issues/new?template=feature_request.md`,
    ].join("\n");

    const missingLabelLine = missingLabel
      ? `- Add one label from: ${TYPE_LABELS.join(", ")}`
      : null;

    const missingSectionsLines =
      missingSections.length > 0
        ? [
            "Missing required sections:",
            ...missingSections.map((section) => `- ${section}`),
          ].join("\n")
        : null;

    const messageParts = [
      "<!-- issue-template-check -->",
      "Thanks for opening this issue. To help us triage it, please provide the missing details:",
      missingLabelLine,
      missingSectionsLines,
      templateLines,
    ].filter(Boolean);

    await updateOrCreateComment(messageParts.join("\n"));
    return;
  }

  if (labelNames.includes(NEEDS_INFO_LABEL)) {
    await github.rest.issues.removeLabel({
      owner,
      repo,
      issue_number: issueNumber,
      name: NEEDS_INFO_LABEL,
    });
  }

  await deleteExistingComment();
}

module.exports = {
  TYPE_LABELS,
  BUG_LABEL,
  ENHANCEMENT_LABELS,
  NEEDS_INFO_LABEL,
  FALLBACK_BUG_REQUIRED,
  FALLBACK_ENHANCEMENT_REQUIRED,
  normalize,
  detectIssueType,
  readTemplateHeadings,
  parseSections,
  hasMeaningfulContent,
  run,
};

