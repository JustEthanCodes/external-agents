package poolside

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/entireio/external-agents/agents/entire-agent-poolside/internal/protocol"
	"gopkg.in/yaml.v3"
)

type rawHook struct {
	SessionID     string          `json:"session_id"`
	Timestamp     string          `json:"timestamp"`
	CWD           string          `json:"cwd"`
	Prompt        string          `json:"prompt"`
	ToolName      string          `json:"tool_name"`
	ToolInput     json.RawMessage `json:"tool_input"`
	ToolResponse  json.RawMessage `json:"tool_response"`
	InitialPrompt string          `json:"initial_prompt"`
	Reason        string          `json:"reason"`
}

func (a *Agent) ParseHook(name string, input []byte) (*protocol.EventJSON, error) {
	input = bytes.TrimSpace(input)
	if len(input) == 0 {
		return nil, nil
	}
	var raw rawHook
	if e := json.Unmarshal(input, &raw); e != nil {
		return nil, fmt.Errorf("parse Poolside hook input: %w", e)
	}
	if raw.SessionID == "" {
		return nil, nil
	}
	if raw.Timestamp == "" {
		raw.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	ref := a.ResolveSessionFile(filepath.Join(protocol.RepoRoot(), ".entire", "tmp", "poolside"), raw.SessionID)
	if e := appendRecord(ref, name, raw, input); e != nil {
		return nil, e
	}
	meta := map[string]string{"agent": "poolside", "hook_event_name": name}
	if raw.CWD != "" {
		meta["cwd"] = raw.CWD
	}
	if raw.Reason != "" {
		meta["reason"] = raw.Reason
	}
	switch name {
	case "SessionStart":
		return &protocol.EventJSON{Type: 1, SessionID: raw.SessionID, SessionRef: ref, Timestamp: raw.Timestamp, Prompt: raw.InitialPrompt, Metadata: meta}, nil
	case "UserPromptSubmit":
		return &protocol.EventJSON{Type: 2, SessionID: raw.SessionID, SessionRef: ref, Timestamp: raw.Timestamp, Prompt: raw.Prompt, Metadata: meta}, nil
	case "Stop":
		return &protocol.EventJSON{Type: 3, SessionID: raw.SessionID, SessionRef: ref, Timestamp: raw.Timestamp, ResponseMessage: extractText(raw.ToolResponse), Metadata: meta}, nil
	case "PreCompact":
		return &protocol.EventJSON{Type: 4, SessionID: raw.SessionID, SessionRef: ref, Timestamp: raw.Timestamp, Metadata: meta}, nil
	default:
		return nil, nil
	}
}
func extractText(b json.RawMessage) string {
	var s string
	if json.Unmarshal(b, &s) == nil {
		return s
	}
	var v map[string]any
	if json.Unmarshal(b, &v) == nil {
		for _, k := range []string{"text", "message", "content", "output"} {
			if s, ok := v[k].(string); ok {
				return s
			}
		}
	}
	return ""
}
func appendRecord(ref, name string, raw rawHook, payload []byte) error {
	if e := os.MkdirAll(filepath.Dir(ref), 0700); e != nil {
		return e
	}
	rec := map[string]any{"event": name, "session_id": raw.SessionID, "timestamp": raw.Timestamp, "cwd": raw.CWD, "prompt": raw.Prompt, "tool_name": raw.ToolName, "tool_input": raw.ToolInput, "tool_response": raw.ToolResponse, "response_message": extractText(raw.ToolResponse)}
	rec["payload"] = json.RawMessage(append([]byte(nil), payload...))
	b, e := json.Marshal(rec)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(ref, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = f.Write(append(b, '\n'))
	return e
}

const poolsideSettingsFile = ".poolside/settings.yaml"

var poolsideHookNames = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", "PreCompact"}

func (a *Agent) InstallHooks(_ bool, _ bool) (int, error) {
	path := filepath.Join(protocol.RepoRoot(), poolsideSettingsFile)
	settings, err := readPoolsideSettings(path)
	if err != nil {
		return 0, err
	}
	hooks, err := settingsMap(settings, "hooks")
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, hookName := range poolsideHookNames {
		entries, err := hookEntries(hooks, hookName)
		if err != nil {
			return 0, err
		}
		command := hookCommand(hookName)
		if !hasHookCommand(entries, command) {
			entries = append(entries, map[string]any{"command": command})
			changed++
		}
		hooks[hookName] = entries
	}
	if changed == 0 {
		return 0, nil
	}
	settings["hooks"] = hooks
	if err := writePoolsideSettings(path, settings); err != nil {
		return 0, err
	}
	return changed, nil
}

func (a *Agent) UninstallHooks() error {
	path := filepath.Join(protocol.RepoRoot(), poolsideSettingsFile)
	settings, err := readPoolsideSettings(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	hooks, err := settingsMap(settings, "hooks")
	if err != nil {
		return err
	}
	changed := false
	for _, hookName := range poolsideHookNames {
		entries, err := hookEntries(hooks, hookName)
		if err != nil {
			return err
		}
		filtered := entries[:0]
		for _, entry := range entries {
			if entryCommand(entry) == hookCommand(hookName) {
				changed = true
				continue
			}
			filtered = append(filtered, entry)
		}
		if len(filtered) == 0 {
			delete(hooks, hookName)
		} else {
			hooks[hookName] = filtered
		}
	}
	if !changed {
		return nil
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}
	return writePoolsideSettings(path, settings)
}

func (a *Agent) AreHooksInstalled() bool {
	settings, err := readPoolsideSettings(filepath.Join(protocol.RepoRoot(), poolsideSettingsFile))
	if err != nil {
		return false
	}
	hooks, err := settingsMap(settings, "hooks")
	if err != nil {
		return false
	}
	for _, hookName := range poolsideHookNames {
		entries, err := hookEntries(hooks, hookName)
		if err != nil || !hasHookCommand(entries, hookCommand(hookName)) {
			return false
		}
	}
	return true
}

func hookCommand(name string) string { return "entire hooks poolside " + name }

func readPoolsideSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	settings := make(map[string]any)
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return settings, nil
}

func settingsMap(settings map[string]any, key string) (map[string]any, error) {
	value, ok := settings[key]
	if !ok {
		return map[string]any{}, nil
	}
	hooks, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Poolside setting %q must be a mapping", key)
	}
	return hooks, nil
}

func hookEntries(hooks map[string]any, name string) ([]any, error) {
	value, ok := hooks[name]
	if !ok {
		return nil, nil
	}
	entries, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("Poolside hook %q must be a list", name)
	}
	return entries, nil
}

func entryCommand(value any) string {
	entry, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	command, _ := entry["command"].(string)
	return command
}

func hasHookCommand(entries []any, command string) bool {
	for _, entry := range entries {
		if entryCommand(entry) == command {
			return true
		}
	}
	return false
}

func writePoolsideSettings(path string, settings map[string]any) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to modify symlinked Poolside settings %s", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".settings.yaml.entire-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}
