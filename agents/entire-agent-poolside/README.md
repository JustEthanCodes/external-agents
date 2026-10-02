# entire-agent-poolside

External agent adapter for [Entire](https://entire.io/) and Poolside's `pool` CLI.

## Status

Preview. The adapter uses Poolside's documented lifecycle hooks to maintain an Entire-owned JSONL transcript. It deliberately does not depend on Poolside's private log/trajectory storage layout.

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

`install-hooks` creates `.poolside/settings.yaml` only when that file does not already exist. It never overwrites an existing Poolside configuration.

If a project already has `.poolside/settings.yaml`, add the six Entire hook commands to its existing `hooks` configuration rather than replacing the file.

## Development

```bash
go test ./...
go build ./cmd/entire-agent-poolside
```

The adapter is intentionally marked preview until it has been exercised against a live Poolside session in the upstream E2E environment.
