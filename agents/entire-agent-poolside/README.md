# entire-agent-poolside

External agent adapter for [Entire](https://entire.io/) and Poolside's `pool` CLI.

## Status

Preview. The adapter uses a hook-backed, Entire-owned JSONL sidecar and does not depend on Poolside's private transcript storage. The Poolside CLI and hook implementation are not part of this repository, so the hook names, payload fields, settings schema, and resume command must be verified against the installed Poolside release before production use.

## Supported lifecycle

- `SessionStart`
- `UserPromptSubmit`
- `PreToolUse`
- `PostToolUse`
- `Stop`
- `PreCompact`

The hook payloads are copied to `.entire/tmp/poolside/<session>.jsonl`. Entire can then read and analyse that stable adapter-owned transcript.

## Installation

```bash
cd agents/entire-agent-poolside
go build -o entire-agent-poolside ./cmd/entire-agent-poolside
cp entire-agent-poolside ~/.local/bin/
```

Then enable Poolside as an external agent in Entire.

## Poolside configuration

`install-hooks` parses `.poolside/settings.yaml`, adds missing Entire command entries under the six configured hook names, and writes the merged YAML atomically. Existing settings and non-Entire hook entries are preserved. Re-running installation is a no-op. `uninstall-hooks` removes only entries whose command exactly matches the adapter's generated command.

The adapter assumes Poolside accepts this shape:

```yaml
hooks:
	SessionStart:
		- command: entire hooks poolside SessionStart
```

That syntax and the command routing are adapter assumptions, not verified behavior bundled with this repository. Malformed settings are rejected and `--force` never overwrites them.

## Development

```bash
go test ./...
go build ./cmd/entire-agent-poolside
```

The adapter is intentionally marked preview until it has been exercised against a live Poolside session. There is no native Poolside transcript integration, token accounting, or transcript compaction implementation. Modified-file extraction is heuristic and only considers path-like values from tool names that look like file mutations.
