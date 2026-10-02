# Poolside — External Agent Research

## Verdict

COMPATIBLE as a hooks-backed adapter; native transcript storage is intentionally not used.

## Evidence

Poolside documents six lifecycle hook events: `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Stop`, `PreCompact`, and `SessionStart`. Hooks are configured under the top-level `hooks` key in project or user `settings.yaml`.

Poolside also documents session persistence and `pool -r` resume, but its public documentation does not define a stable on-disk transcript schema. The adapter therefore treats Poolside hook payloads as the integration boundary and stores a normalized JSONL copy under Entire's temporary session directory.

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

## Limitations

Poolside does not document a true session-end lifecycle hook; `Stop` is a turn boundary. The adapter consequently does not synthesize a session-end event.

Token accounting and native transcript compaction are not declared capabilities because the hook protocol does not provide a stable token ledger or native transcript schema.

## Security

Hook payloads can contain prompts, tool arguments, and tool results. The adapter writes them with restrictive file permissions under `.entire/tmp`.
