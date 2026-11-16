#!/usr/bin/env node
// export-log.mjs — saves Claude Code conversations into the repo (ai-logs/).
//
// Claude Code runs this script as a hook (see .claude/settings.json) on the
// Stop, SubagentStop, PreCompact and SessionEnd events. The hook passes a JSON
// object on stdin that tells us where the session transcript lives on disk.
//
// Every run re-exports the WHOLE transcript, so the output files are simply
// overwritten with a more complete version each time (idempotent, no state).
//
// For each transcript we write two files:
//   ai-logs/raw/<session_id>.jsonl      machine-readable copy (sanitised)
//   ai-logs/<date>_<session_id>.md      human-readable version
// Subagent transcripts go to ai-logs/raw/subagents/ and ai-logs/subagents/.
//
// Manual use (backfill):
//   node .claude/hooks/export-log.mjs --all                  every session of this project
//   node .claude/hooks/export-log.mjs --transcript <path>    one transcript file
//
// No external dependencies: only Node.js built-ins.

import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// The script lives in <project>/.claude/hooks/, so the project root is two levels up.
// Deriving it from the script location works no matter which directory the hook runs in.
const PROJECT_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const LOG_DIR = path.join(PROJECT_DIR, 'ai-logs');
const RAW_DIR = path.join(LOG_DIR, 'raw');
const ERROR_LOG = path.join(LOG_DIR, 'hook-errors.log');

const MAX_RESULT_CHARS = 2000; // tool results are truncated to this in the Markdown
const MAX_INPUT_CHARS = 10000; // a single tool input value (e.g. a whole written file)

// ---------------------------------------------------------------------------
// Secret redaction
// ---------------------------------------------------------------------------

// Patterns for values that look like secrets wherever they appear.
// Each entry is [regular expression, replacement].
const SECRET_PATTERNS = [
  // Stripe API keys: sk_test_..., sk_live_..., pk_test_..., rk_live_...
  [/\b(?:sk|pk|rk)_(?:test|live)_[A-Za-z0-9]{8,}/g, '[REDACTED:stripe-key]'],
  // Stripe webhook signing secret
  [/\bwhsec_[A-Za-z0-9+/=]{8,}/g, '[REDACTED:stripe-webhook-secret]'],
  // JSON Web Tokens: three base64url parts, the first two start with "eyJ"
  [/\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}/g, '[REDACTED:jwt]'],
  // GitHub tokens
  [/\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})/g, '[REDACTED:github-token]'],
  // Anthropic API keys
  [/\bsk-ant-[A-Za-z0-9_-]{10,}/g, '[REDACTED:anthropic-key]'],
  // Claude account / organisation ids that show up when a transcript file itself is printed
  [/((?:account|organization)Uuid\\*"\s*:\s*\\*")[0-9a-f-]{36}/gi, '$1[REDACTED:account-id]'],
  // Passwords inside connection strings: postgres://user:PASSWORD@host/db
  [/\b([a-z][a-z0-9+.-]*:\/\/[^\s:/@"']+):([^\s@"'/]+)@/gi, '$1:[REDACTED:password]@'],
  // ENV-style assignments whose name says it is secret: JWT_SECRET=abc123
  [
    /\b([A-Z0-9_]*(?:SECRET|PASSWORD|PASSWD|TOKEN|API_KEY|PRIVATE_KEY)[A-Z0-9_]*)(\s*=\s*)(["']?)([^\s"']{6,})/g,
    '$1$2$3[REDACTED]',
  ],
];

const SKIP_DIRS = new Set(['node_modules', '.git', 'ai-logs', 'dist', 'build', 'vendor']);
const EXAMPLE_ENV = /\.(example|sample|template)$/i;

// Parses KEY=VALUE lines of a .env file into [key, value] pairs.
function parseEnvFile(file) {
  const pairs = [];
  for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const match = line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/);
    if (!match) continue;
    const value = match[2].trim().replace(/^(["'])(.*)\1$/, '$2'); // strip surrounding quotes
    pairs.push([match[1], value]);
  }
  return pairs;
}

// Finds .env files in the project (max 3 folders deep) and returns the literal
// values that must be hidden, as [key, value] pairs.
// Values that also appear in a committed .env.example are public by definition
// and are not redacted (otherwise "localhost" etc. would be blanked everywhere).
function loadEnvSecrets() {
  const secret = [];
  const publicValues = new Set();

  const walk = (dir, depth) => {
    let entries = [];
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return;
    }
    for (const entry of entries) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (depth < 3 && !SKIP_DIRS.has(entry.name)) walk(full, depth + 1);
      } else if (entry.name === '.env' || entry.name.startsWith('.env.')) {
        try {
          const pairs = parseEnvFile(full);
          if (EXAMPLE_ENV.test(entry.name)) pairs.forEach(([, value]) => publicValues.add(value));
          else secret.push(...pairs);
        } catch {
          /* unreadable file: ignore */
        }
      }
    }
  };
  walk(PROJECT_DIR, 0);

  return secret
    .filter(([, value]) => value.length >= 6 && !publicValues.has(value))
    .sort((a, b) => b[1].length - a[1].length); // longest first, so substrings do not break longer values
}

function makeRedactor() {
  const envSecrets = loadEnvSecrets();
  return (text) => {
    for (const [key, value] of envSecrets) text = text.split(value).join(`[REDACTED:${key}]`);
    for (const [pattern, replacement] of SECRET_PATTERNS) text = text.replace(pattern, replacement);
    return text;
  };
}

// ---------------------------------------------------------------------------
// Sanitising transcript records
// ---------------------------------------------------------------------------

// A transcript line is one JSON object ("record"). Besides the conversation,
// Claude Code stores internal bookkeeping in the same file (cached API
// requests, file snapshots, account ids...). We keep only the conversation.
const KEEP_TYPES = new Set(['user', 'assistant', 'system', 'summary']);
const KEEP_KEYS = [
  'type', 'subtype', 'uuid', 'parentUuid', 'timestamp', 'sessionId', 'agentId',
  'isSidechain', 'isMeta', 'isCompactSummary', 'level', 'cwd', 'version', 'gitBranch',
  'message', 'content', 'summary',
];
const THINKING_TYPES = new Set(['thinking', 'redacted_thinking']);

// Recursively copies a JSON value while:
//  - dropping thinking/reasoning blocks,
//  - replacing embedded base64 files (images, PDFs) with a short placeholder,
//  - redacting secrets in every string.
function clean(value, redact) {
  if (typeof value === 'string') return redact(value);
  if (Array.isArray(value)) {
    return value
      .filter((item) => !(item && typeof item === 'object' && THINKING_TYPES.has(item.type)))
      .map((item) => clean(item, redact));
  }
  if (value && typeof value === 'object') {
    const out = {};
    for (const [key, child] of Object.entries(value)) {
      if (key === 'signature') continue; // cryptographic signature of a thinking block
      if (key === 'data' && value.type === 'base64' && typeof child === 'string') {
        out[key] = `[base64 omitted: ${child.length} chars]`;
      } else {
        out[key] = clean(child, redact);
      }
    }
    return out;
  }
  return value;
}

// Reads a transcript and returns { records, title }.
// records = sanitised conversation records, in file order.
function readTranscript(file, redact) {
  const records = [];
  let title = '';
  for (const line of fs.readFileSync(file, 'utf8').split('\n')) {
    if (!line.trim()) continue;
    let record;
    try {
      record = JSON.parse(line);
    } catch {
      continue; // the last line may be half-written while the session is live
    }
    if (record.type === 'ai-title' && record.aiTitle) title = redact(String(record.aiTitle));
    if (!KEEP_TYPES.has(record.type)) continue;

    const slim = {};
    for (const key of KEEP_KEYS) if (key in record) slim[key] = clean(record[key], redact);

    // An assistant record that contained only thinking is now empty: skip it.
    const content = slim.message?.content;
    if (slim.type === 'assistant' && Array.isArray(content) && content.length === 0) continue;

    records.push(slim);
  }
  return { records, title };
}

// ---------------------------------------------------------------------------
// Markdown rendering
// ---------------------------------------------------------------------------

// Wraps text in a code fence that is longer than any backtick run inside it,
// so content that itself contains ``` cannot break out of the block.
function fence(text, lang = '') {
  const longestRun = Math.max(0, ...(text.match(/`+/g) || []).map((run) => run.length));
  const ticks = '`'.repeat(Math.max(3, longestRun + 1));
  return `${ticks}${lang}\n${text}\n${ticks}`;
}

function truncate(text, max) {
  if (text.length <= max) return text;
  return `${text.slice(0, max)}\n… [truncated: ${text.length - max} more characters, full text in ai-logs/raw/]`;
}

function formatTime(timestamp) {
  if (!timestamp) return '';
  return `${String(timestamp).replace('T', ' ').slice(0, 16)} UTC`;
}

function blockquote(text) {
  return text.split('\n').map((line) => `> ${line}`.trimEnd()).join('\n');
}

// Content is either a plain string or an array of blocks ({type:'text'}, {type:'image'}, ...).
// Returns the visible text.
function contentToText(content) {
  if (typeof content === 'string') return content;
  if (!Array.isArray(content)) return '';
  return content
    .map((block) => {
      if (block.type === 'text') return block.text;
      if (block.type === 'image') return '[image]';
      if (block.type === 'document') return '[document]';
      return '';
    })
    .filter(Boolean)
    .join('\n');
}

// Slash commands are stored wrapped in XML-like tags; show them in plain words.
function describeLocalCommand(text) {
  const name = text.match(/<command-name>([\s\S]*?)<\/command-name>/);
  if (name) {
    const args = text.match(/<command-args>([\s\S]*?)<\/command-args>/);
    return `Slash command: \`${`${name[1].trim()} ${args ? args[1].trim() : ''}`.trim()}\``;
  }
  const output = text.match(/<local-command-(?:stdout|stderr)>([\s\S]*?)<\/local-command-(?:stdout|stderr)>/);
  if (output) return `Command output: ${output[1].trim()}`;
  return null;
}

function renderToolCall(block, result) {
  const lines = [`**🔧 Tool call: \`${block.name}\`**`, ''];
  for (const [key, value] of Object.entries(block.input || {})) {
    const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
    if (text.includes('\n') || text.length > 100) {
      lines.push(`- **${key}:**`, '', fence(truncate(text, MAX_INPUT_CHARS)), '');
    } else {
      lines.push(`- **${key}:** \`${text}\``);
    }
  }
  if (result) {
    const text = contentToText(result.content) || '(empty)';
    const label = result.is_error ? 'Result (error)' : 'Result';
    lines.push('', `<details><summary>${label}</summary>`, '', fence(truncate(text, MAX_RESULT_CHARS)), '', '</details>');
  } else {
    lines.push('', '*(no result recorded)*');
  }
  return lines.join('\n');
}

function renderMarkdown(records, info) {
  // Tool results arrive in a later "user" record; index them by the id of the
  // tool call they answer, so each result can be shown right under its call.
  const results = new Map();
  for (const record of records) {
    const content = record.message?.content;
    if (record.type !== 'user' || !Array.isArray(content)) continue;
    for (const block of content) if (block.type === 'tool_result') results.set(block.tool_use_id, block);
  }

  const out = [
    `# ${info.heading}`,
    '',
    `- **Session ID:** \`${info.sessionId}\``,
    ...(info.title ? [`- **Title:** ${info.title}`] : []),
    `- **Started:** ${formatTime(records[0]?.timestamp)}`,
    `- **Last activity:** ${formatTime(records[records.length - 1]?.timestamp)}`,
    '',
    '> Generated automatically by `.claude/hooks/export-log.mjs` (see `ai-logs/README.md`).',
    '> Contains every user prompt, every visible assistant response and every tool call.',
    '> Model thinking/reasoning blocks are not included; secrets are redacted; tool results are truncated.',
    '',
  ];

  let speaker = ''; // who "spoke" last, so consecutive assistant records share one heading
  const heading = (who, label, timestamp) => {
    if (speaker !== who) out.push('---', '', `## ${label}  ·  ${formatTime(timestamp)}`, '');
    speaker = who;
  };
  const note = (text) => {
    out.push(blockquote(`ℹ️ ${text}`), '');
    speaker = '';
  };

  for (const record of records) {
    const content = record.message?.content;

    if (record.type === 'system') {
      if (record.subtype === 'compact_boundary') note('Context was compacted here (older messages summarised for the model; the log above is still complete).');
      else if (record.subtype === 'local_command') note(describeLocalCommand(String(record.content)) || String(record.content));
      else if (typeof record.content === 'string' && record.content.trim() && record.subtype !== 'turn_duration') note(`System: ${record.content}`);
      continue;
    }

    if (record.type === 'summary') {
      if (record.summary) note(`Session summary: ${record.summary}`);
      continue;
    }

    if (record.type === 'user') {
      const text = contentToText(content).trim();
      if (!text) continue; // record held only tool results (already shown under their calls)

      const command = describeLocalCommand(text);
      if (text.startsWith('<local-command-caveat>')) continue; // boilerplate that precedes slash commands
      if (command) {
        note(command);
      } else if (record.isMeta || record.isCompactSummary) {
        // Text inserted by Claude Code itself (skill instructions, compaction summaries...), not typed by the user.
        const label = record.isCompactSummary ? 'Compaction summary (generated by Claude Code)' : 'Message injected by Claude Code (not typed by the user)';
        out.push(`<details><summary>${label}</summary>`, '', fence(text), '', '</details>', '');
        speaker = '';
      } else {
        heading(`user-${record.uuid}`, '👤 User', record.timestamp); // every prompt gets its own heading
        out.push(blockquote(text), '');
      }
      continue;
    }

    if (record.type === 'assistant' && Array.isArray(content)) {
      for (const block of content) {
        if (block.type === 'text' && block.text.trim()) {
          heading('assistant', '🤖 Assistant', record.timestamp);
          out.push(block.text.trim(), '');
        } else if (block.type === 'tool_use') {
          heading('assistant', '🤖 Assistant', record.timestamp);
          out.push(renderToolCall(block, results.get(block.id)), '');
        }
      }
    }
  }

  return out.join('\n');
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

// Local calendar date (YYYY-MM-DD) of the first message; used in the file name.
function sessionDate(records) {
  const first = records.find((record) => record.timestamp);
  const date = first ? new Date(first.timestamp) : new Date();
  const pad = (n) => String(n).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function writeFile(file, text) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, text);
}

// Exports one transcript file. `agentName` is set for subagent transcripts.
// Returns the path of the Markdown file (or null if there was nothing to export).
function exportTranscript(transcriptPath, sessionId, redact, agentName = null) {
  const { records, title } = readTranscript(transcriptPath, redact);
  if (records.length === 0) return null;

  const rawName = agentName ? path.join('subagents', `${sessionId}_${agentName}.jsonl`) : `${sessionId}.jsonl`;
  writeFile(path.join(RAW_DIR, rawName), records.map((record) => JSON.stringify(record)).join('\n') + '\n');

  const date = sessionDate(records);
  const mdName = agentName ? path.join('subagents', `${date}_${sessionId}_${agentName}.md`) : `${date}_${sessionId}.md`;
  const heading = agentName ? `Subagent log: ${agentName}` : 'Claude Code session log';
  const mdPath = path.join(LOG_DIR, mdName);
  writeFile(mdPath, renderMarkdown(records, { heading, sessionId, title }) + '\n');
  return mdPath;
}

// Exports a session and all of its subagents. Returns the main Markdown path.
function exportSession(transcriptPath, sessionId, redact, extraAgentTranscript = null) {
  const mdPath = exportTranscript(transcriptPath, sessionId, redact);

  // Subagent transcripts live next to the main one: <dir>/<session_id>/subagents/agent-<id>.jsonl
  const agentFiles = new Set();
  const agentDir = path.join(path.dirname(transcriptPath), sessionId, 'subagents');
  if (fs.existsSync(agentDir)) {
    for (const name of fs.readdirSync(agentDir)) if (name.endsWith('.jsonl')) agentFiles.add(path.join(agentDir, name));
  }
  if (extraAgentTranscript && fs.existsSync(extraAgentTranscript)) agentFiles.add(path.resolve(extraAgentTranscript));

  for (const file of agentFiles) exportTranscript(file, sessionId, redact, path.basename(file, '.jsonl'));
  return mdPath;
}

// "~/..." paths are not expanded by Node, so do it ourselves.
function expandHome(file) {
  return file && file.startsWith('~') ? path.join(os.homedir(), file.slice(1)) : file;
}

// Claude Code stores transcripts in ~/.claude/projects/<project path with every
// non-alphanumeric character replaced by "-">/
function claudeTranscriptDir() {
  return path.join(os.homedir(), '.claude', 'projects', PROJECT_DIR.replace(/[^A-Za-z0-9]/g, '-'));
}

function readHookInput() {
  if (process.stdin.isTTY) return {}; // started by hand without piped input
  try {
    return JSON.parse(fs.readFileSync(0, 'utf8')); // file descriptor 0 = stdin
  } catch {
    return {};
  }
}

function main() {
  const args = process.argv.slice(2);
  const redact = makeRedactor();
  const exported = [];

  if (args.includes('--all')) {
    const dir = claudeTranscriptDir();
    for (const name of fs.readdirSync(dir)) {
      if (name.endsWith('.jsonl')) exported.push(exportSession(path.join(dir, name), path.basename(name, '.jsonl'), redact));
    }
  } else {
    const flagIndex = args.indexOf('--transcript');
    const input = flagIndex >= 0 ? { transcript_path: args[flagIndex + 1] } : readHookInput();
    const transcriptPath = expandHome(input.transcript_path);
    if (!transcriptPath) throw new Error('no transcript_path in hook input');
    // Short-lived CLI commands (e.g. `claude plugin list`) fire SessionEnd without ever
    // writing a transcript. There is no conversation to export, so this is not an error.
    if (!fs.existsSync(transcriptPath)) return;
    const sessionId = input.session_id || path.basename(transcriptPath, '.jsonl');
    exported.push(exportSession(transcriptPath, sessionId, redact, expandHome(input.agent_transcript_path)));
  }

  // Print the result only for manual runs; as a hook we stay silent so nothing
  // is mistaken for hook output.
  if (process.stdin.isTTY || args.length > 0) exported.filter(Boolean).forEach((file) => console.log(file));
}

try {
  main();
} catch (error) {
  // Never fail loudly: a logging problem must not interrupt the coding session.
  try {
    fs.mkdirSync(LOG_DIR, { recursive: true });
    fs.appendFileSync(ERROR_LOG, `${new Date().toISOString()} ${error?.stack || error}\n`);
  } catch {
    /* nothing more we can do */
  }
}
process.exit(0);
