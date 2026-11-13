"use strict";

const fs = require("fs");
const path = require("path");

const LABELS = {
  TYPE: ["kind/bug", "kind/feature", "kind/cleanup"],
  BUG: "kind/bug",
  ENHANCEMENT: ["kind/feature", "kind/cleanup"],
  NEEDS_INFO: "triage/needs-info",
  NEEDS_INFO_COLOR: "d876e3",
  NEEDS_INFO_DESCRIPTION:
    "Blocked pending more info from the reporter (repro, version, logs) before triage.",
};

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

function normalize(title) {
  return title
    .toLowerCase()
    .replace(/[^\w\s]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function parseSections(body) {
  const sections = new Map();
  if (!body) return sections;

  const lines = body.split(/\r?\n/);
  let currentKey = null;
  let buffer = [];

  const flush = () => {
    if (currentKey) {
      sections.set(currentKey, buffer.join("\n").trim());
    }
    buffer = [];
  };

  for (const line of lines) {
    const trimmed = line.trim();
    const headingMatch = trimmed.match(/^#{1,6}\s*(.+?)\s*$/);
    const boldMatch = trimmed.match(/^\*\*(.+?)\*\*$/);
    const colonMatch = trimmed.match(/^(.+?):\s*$/);
    const title = headingMatch?.[1] || boldMatch?.[1] || colonMatch?.[1];

    if (title) {
      flush();
      currentKey = normalize(title);
      continue;
    }
    if (currentKey) {
      buffer.push(line);
    }
  }

  flush();
  return sections;
}

function hasMeaningfulContent(content) {
  if (!content) return false;
  const cleaned = content
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/[\s*-]+/g, " ")
    .trim()
    .toLowerCase();
  if (!cleaned) return false;
  return !["n/a", "na", "none", "tbd", "todo"].includes(cleaned);
}

function detectTemplate(body) {
  if (!body) return null;
  if (body.includes("<!-- template: bug_report -->")) return "bug_report";
  if (body.includes("<!-- template: feature_request -->"))
    return "feature_request";
  return null;
}

function detectIssueType(body) {
  if (!body) return null;
  const normalizedBody = body.toLowerCase();

  if (
    normalizedBody.includes("steps to reproduce") ||
    normalizedBody.includes("expected behavior") ||
    normalizedBody.includes("actual behavior")
  ) {
    return LABELS.BUG;
  }

  if (
    normalizedBody.includes("breaking change") ||
    normalizedBody.includes("scope of the feature")
  ) {
    return "kind/feature";
  }

  return null;
}

function readTemplateHeadings(fileName) {
  try {
    const templatePath = path.join(
      process.env.GITHUB_WORKSPACE || process.cwd(),
      ".github",
      "ISSUE_TEMPLATE",
      fileName,
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
}

const SKIP_LABELS = ["epic", "roadmap", "release"];
const PRIVILEGED_AUTHORS = ["OWNER", "MEMBER", "COLLABORATOR"];

function evaluateIssue({ title, body, labels, authorAssociation, templates }) {
  const labelNames = (labels || [])
    .map((label) => (typeof label === "string" ? label : label.name))
    .filter(Boolean)
    .map((label) => label.toLowerCase());

  const hasManualTriageLabel = labelNames.some(
    (l) => l.startsWith("triage/") && l !== LABELS.NEEDS_INFO,
  );
  const hasSkipLabel = SKIP_LABELS.some((s) => labelNames.includes(s));
  const isPrivilegedAuthor = PRIVILEGED_AUTHORS.includes(authorAssociation);

  let hasTypeLabel = LABELS.TYPE.some((l) => labelNames.includes(l));
  let isBug = labelNames.includes(LABELS.BUG);
  let isEnhancement = LABELS.ENHANCEMENT.some((l) => labelNames.includes(l));

  const template = detectTemplate(body);

  let typeToApply = null;
  if (!hasTypeLabel && template) {
    if (template === "bug_report") typeToApply = LABELS.BUG;
    else if (template === "feature_request") typeToApply = "kind/feature";

    if (typeToApply) {
      hasTypeLabel = true;
      isBug = typeToApply === LABELS.BUG;
      isEnhancement = !isBug;
    }
  }

  if (!hasTypeLabel && !template) {
    const detected = detectIssueType(body);
    if (detected) {
      typeToApply = detected;
      hasTypeLabel = true;
      isBug = detected === LABELS.BUG;
      isEnhancement = !isBug;
    }
  }

  if (!template || hasManualTriageLabel || hasSkipLabel || isPrivilegedAuthor) {
    return {
      typeToApply,
      addNeedsInfo: false,
      removeNeedsInfo: false,
      missingSections: [],
      comment: null,
      deleteComment: !template && !hasManualTriageLabel,
    };
  }

  const bugRequired =
    templates?.bugHeadings && templates.bugHeadings.length > 0
      ? templates.bugHeadings
      : FALLBACK_BUG_REQUIRED;
  const enhancementRequired =
    templates?.featureHeadings && templates.featureHeadings.length > 0
      ? templates.featureHeadings
      : FALLBACK_ENHANCEMENT_REQUIRED;

  const requiredSections = isBug
    ? bugRequired
    : isEnhancement
      ? enhancementRequired
      : [];

  const sections = parseSections(body || "");
  const missingSections = requiredSections.filter((section) => {
    const content = sections.get(normalize(section));
    return !hasMeaningfulContent(content);
  });

  const needsInfo = missingSections.length > 0;

  let comment = null;
  if (needsInfo) {
    const parts = [
      "<!-- issue-template-check -->",
      "Thanks for opening this issue. To help us triage it, please provide the missing details:",
      "Missing required sections:",
    ];
    for (const section of missingSections) {
      parts.push(`- ${section}`);
    }
    parts.push("Issue templates:");
    parts.push("- Bug report: **bug_report.md**");
    parts.push(
      "- Feature request (use for features, improvements, or cleanup): **feature_request.md**",
    );
    comment = parts.join("\n");
  }

  return {
    typeToApply,
    addNeedsInfo: needsInfo && !labelNames.includes(LABELS.NEEDS_INFO),
    removeNeedsInfo: !needsInfo && labelNames.includes(LABELS.NEEDS_INFO),
    missingSections,
    comment: needsInfo ? comment : null,
    deleteComment: !needsInfo,
  };
}

function formatComment(raw, owner, repo) {
  return raw
    .replace(
      /\*\*bug_report\.md\*\*/,
      `[bug_report.md](https://github.com/${owner}/${repo}/issues/new?template=bug_report.md)`,
    )
    .replace(
      /\*\*feature_request\.md\*\*/,
      `[feature_request.md](https://github.com/${owner}/${repo}/issues/new?template=feature_request.md)`,
    );
}

async function run({ github, context }) {
  const issue = context.payload.issue;
  const issueNumber = issue.number;
  const owner = context.repo.owner;
  const repo = context.repo.repo;

  const result = evaluateIssue({
    title: issue.title,
    body: issue.body,
    labels: issue.labels,
    authorAssociation: issue.author_association,
    templates: {
      bugHeadings: readTemplateHeadings("bug_report.md"),
      featureHeadings: readTemplateHeadings("feature_request.md"),
    },
  });

  if (result.typeToApply) {
    await github.rest.issues.addLabels({
      owner,
      repo,
      issue_number: issueNumber,
      labels: [result.typeToApply],
    });
  }

  const findBotComment = async () => {
    const comments = await github.paginate(
      github.rest.issues.listComments,
      { owner, repo, issue_number: issueNumber, per_page: 100 },
    );
    return comments.find(
      (c) =>
        c.user?.type === "Bot" &&
        c.body?.includes("<!-- issue-template-check -->"),
    );
  };

  if (result.addNeedsInfo) {
    try {
      await github.rest.issues.getLabel({ owner, repo, name: LABELS.NEEDS_INFO });
    } catch (error) {
      if (error.status !== 404) throw error;
      await github.rest.issues.createLabel({
        owner,
        repo,
        name: LABELS.NEEDS_INFO,
        color: LABELS.NEEDS_INFO_COLOR,
        description: LABELS.NEEDS_INFO_DESCRIPTION,
      });
    }

    await github.rest.issues.addLabels({
      owner,
      repo,
      issue_number: issueNumber,
      labels: [LABELS.NEEDS_INFO],
    });

    const body = formatComment(result.comment, owner, repo);
    const existing = await findBotComment();
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
        issue_number: issueNumber,
        body,
      });
    }
    return;
  }

  if (result.removeNeedsInfo) {
    await github.rest.issues.removeLabel({
      owner,
      repo,
      issue_number: issueNumber,
      name: LABELS.NEEDS_INFO,
    });
  }

  if (result.deleteComment) {
    const existing = await findBotComment();
    if (existing) {
      await github.rest.issues.deleteComment({
        owner,
        repo,
        comment_id: existing.id,
      });
    }
  }
}

module.exports = {
  LABELS,
  normalize,
  parseSections,
  hasMeaningfulContent,
  detectTemplate,
  detectIssueType,
  evaluateIssue,
  readTemplateHeadings,
  run,
};
