# Poolside — External Agent Research

## Scope and verification status

This adapter is a hooks-backed integration; it intentionally does not read Poolside's private transcript storage. The repository does not include Poolside CLI source or a pinned external fixture, so claims about Poolside behavior below are split between implementation facts and assumptions that require live verification.

## Adapter facts

- The binary detects `pool` with `exec.LookPath`.
- Hook installation assumes `.poolside/settings.yaml` has a top-level `hooks` mapping whose event values are lists of `{command: ...}` mappings.
- The adapter stores the complete validated hook JSON, plus normalized fields, under `.entire/tmp/poolside/<safe-session-id>.jsonl`.
- Session references supplied through the protocol are restricted to that adapter-owned directory.
- Installation merges YAML atomically, is idempotent, preserves non-Entire entries, rejects malformed settings, and uninstallation removes only exact generated command entries.

## External assumptions requiring live verification

- The six configured hook names are accepted by the installed Poolside release: `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Stop`, `PreCompact`, and `SessionStart`.
- Poolside emits `session_id`, `timestamp`, `cwd`, prompt, tool, and response fields with the names represented by `rawHook`.
- `entire hooks poolside <HookName>` is the correct hook command form.
- `pool -r <session-id>` is the correct resume syntax.
- Poolside's settings YAML accepts the command-list shape shown in the README.

## Session model

- Session ID: `session_id` from hook payloads.
- Session directory: `.entire/tmp/poolside/`.
- Session file: safe session ID + `.jsonl`.
- Resume: `pool -r <session-id>`.
- Transcript: adapter-owned JSONL of received hook payloads.

## Protocol mapping

| Poolside event | Entire event |
| --- | --- |
| SessionStart | SessionStart (1) |
| UserPromptSubmit | TurnStart (2) |
| Stop | TurnEnd (3) |
| PreCompact | PreCompact (4) |
| PreToolUse | captured for transcript analysis |
| PostToolUse | captured for transcript analysis |

## Entire mapping and limitations

The adapter has no session-end mapping; `Stop` is treated as a turn boundary and no session-end event is synthesized.

Token accounting and native transcript compaction are not declared capabilities because this adapter has no verified Poolside token ledger or native transcript schema. Pre/post tool events are retained in the sidecar for analysis but do not produce standalone Entire events. Modified-file extraction is heuristic and may require adjustment once real Poolside tool names and payloads are verified.

## Security

Hook payloads can contain prompts, tool arguments, and tool results. The adapter writes them with restrictive file permissions under `.entire/tmp`.
