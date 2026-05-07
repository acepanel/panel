/*
 Language: systemd Journal (journalctl)
 Description: systemd/journald logs (journalctl -o short / short-iso)
 Category: system, logs
 */

/** @type {import('highlight.js').LanguageFn} */
export default function systemdJournal(hljs: any) {
  const regex = hljs.regex

  // Month names for "short" format
  const MONTH = '(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)'

  // journalctl -o short:
  // Jan 13 10:22:33 ...
  const TS_SHORT = new RegExp(`^${MONTH}\\s+\\d{1,2}\\s+\\d{2}:\\d{2}:\\d{2}`)

  // journalctl -o short-iso:
  // 2026-01-13T10:22:33+0800 ...
  // 2026-01-13 10:22:33 ...
  const TS_ISO = /^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?/

  const HOST = /[A-Za-z0-9][A-Za-z0-9_.-]*/
  const IDENT = /[A-Za-z_][A-Za-z0-9_.-]*(?:\/[A-Za-z0-9_.-]+)*/ // e.g. systemd, sshd, foo/bar
  const PID_OPT = /(?:\[\d+\])?/

  const UNIT = /\b[\w.-]+?\.(?:service|socket|target|timer|mount|device|path|slice|scope)\b/

  const PRIORITY_WORDS = [
    'emerg',
    'alert',
    'crit',
    'critical',
    'err',
    'error',
    'warn',
    'warning',
    'notice',
    'info',
    'debug',
    'panic',
    // systemd/journal 常见状态词
    'failed',
    'failure',
    'timeout',
    'timed',
    'denied',
    'refused',
    'segfault',
  ]

  const SYSTEMD_VERBS = [
    'starting',
    'started',
    'stopping',
    'stopped',
    'reloading',
    'reloaded',
    'restarting',
    'restarted',
    'activating',
    'activated',
    'deactivating',
    'deactivated',
    'mounted',
    'mounting',
    'unmounted',
    'unmounting',
    'listening',
    'triggered',
    'queued',
    'succeeded',
    'success',
  ]

  // Prefix: "<timestamp> <host> <ident>[pid]: "
  // Example:
  // Jan 13 10:22:33 host systemd[1]:
  // 2026-01-13T10:22:33+0800 host sshd[1234]:
  const PREFIX = {
    begin: [regex.either(TS_ISO, TS_SHORT), /\s+/, HOST, /\s+/, IDENT, PID_OPT],
    beginScope: {
      1: 'meta', // timestamp
      3: 'title', // hostname
      5: 'symbol', // identifier
      6: 'number', // [pid]
    },
    end: /: /,
    endScope: 'punctuation',
    relevance: 10,
  }

  const SEVERITY = {
    match: new RegExp(`\\b(?:${PRIORITY_WORDS.join('|')})\\b`, 'i'),
    scope: 'keyword',
    relevance: 2,
  }

  const STATUS_VERB = {
    match: new RegExp(`\\b(?:${SYSTEMD_VERBS.join('|')})\\b`, 'i'),
    scope: 'built_in',
    relevance: 1,
  }

  // key=value（如：code=exited status=1/FAILURE UNIT=foo.service）
  const KEY_VALUE = {
    begin: /\b[\w.-]+=/,
    scope: 'attr',
    relevance: 0,
  }

  // kernel/journal 常见的 monotonic timestamp: "[ 123.456]"
  const MONOTONIC = {
    match: /\[\s*\d+(?:\.\d+)?\]/,
    scope: 'meta',
    relevance: 0,
  }

  const DQUOTE = {
    scope: 'string',
    begin: /"/,
    end: /"/,
    illegal: /\n/,
    relevance: 0,
  }

  const SQUOTE = {
    scope: 'string',
    begin: /'/,
    end: /'/,
    illegal: /\n/,
    relevance: 0,
  }

  const PATH = {
    match: /(?:\/[^\s"'():[\]]+)+/,
    scope: 'string',
    relevance: 0,
  }

  return {
    name: 'systemd Journal',
    aliases: ['journalctl', 'journald', 'systemdlog', 'systemd-journal', 'systemd'],
    case_insensitive: true,
    contains: [
      PREFIX,

      // unit names in message body
      { match: UNIT, scope: 'title', relevance: 3 },

      // severity/status words
      SEVERITY,
      STATUS_VERB,

      MONOTONIC,
      KEY_VALUE,

      // numbers (exit codes, pids, etc.)
      hljs.NUMBER_MODE,

      // strings/paths
      DQUOTE,
      SQUOTE,
      PATH,
    ],
  }
}
