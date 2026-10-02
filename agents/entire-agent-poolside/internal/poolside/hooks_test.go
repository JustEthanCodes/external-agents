package poolside

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseHook(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ENTIRE_REPO_ROOT", root)
	a := New()
	input := map[string]any{"session_id": "sess-1", "timestamp": "2026-10-02T13:00:00Z", "cwd": root, "prompt": "hello"}
	b, _ := json.Marshal(input)
	ev, e := a.ParseHook("UserPromptSubmit", b)
	if e != nil {
		t.Fatal(e)
	}
	if ev == nil || ev.Type != 2 || ev.SessionID != "sess-1" || ev.Prompt != "hello" {
		t.Fatalf("unexpected event: %#v", ev)
	}
	ref := filepath.Join(root, ".entire", "tmp", "poolside", "sess-1.jsonl")
	if _, e := os.Stat(ref); e != nil {
		t.Fatalf("transcript not written: %v", e)
	}
}
func TestEmptyHookIsIgnored(t *testing.T) {
	if ev, e := New().ParseHook("UserPromptSubmit", nil); e != nil || ev != nil {
		t.Fatalf("got %#v %v", ev, e)
	}
}

func TestDetectUsesPoolBinary(t *testing.T) {
	agent := New()
	agent.LookPath = func(name string) (string, error) {
		if name != "pool" {
			t.Fatalf("looked up %q", name)
		}
		return "/usr/bin/pool", nil
	}
	if !agent.Detect().Present {
		t.Fatal("expected Poolside to be detected")
	}
	agent.LookPath = func(string) (string, error) { return "", errors.New("missing") }
	if agent.Detect().Present {
		t.Fatal("expected Poolside to be absent")
	}
}

func TestParseHookSupportedEventsAndTranscriptPosition(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ENTIRE_REPO_ROOT", root)
	agent := New()
	payload := []byte(`{"session_id":"session/../unsafe","timestamp":"2026-10-02T13:00:00Z","cwd":"/tmp/untrusted","prompt":"hello","initial_prompt":"start","tool_name":"write_file","tool_input":{"file_path":"a.txt"},"tool_response":"done"}`)
	tests := []struct {
		name   string
		typeID int
		prompt string
	}{
		{"SessionStart", 1, "start"},
		{"UserPromptSubmit", 2, "hello"},
		{"PreToolUse", 0, ""},
		{"PostToolUse", 0, ""},
		{"Stop", 3, ""},
		{"PreCompact", 4, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, err := agent.ParseHook(test.name, payload)
			if err != nil {
				t.Fatal(err)
			}
			if test.typeID == 0 {
				if event != nil {
					t.Fatalf("expected no Entire event, got %#v", event)
				}
				return
			}
			if event == nil || event.Type != test.typeID || event.Prompt != test.prompt {
				t.Fatalf("unexpected event: %#v", event)
			}
		})
	}
	files, err := os.ReadDir(filepath.Join(root, ".entire", "tmp", "poolside"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one safe transcript, files=%v err=%v", files, err)
	}
	if strings.Contains(files[0].Name(), string(filepath.Separator)+"..") {
		t.Fatalf("unsafe transcript filename %q", files[0].Name())
	}
	position, err := agent.GetTranscriptPosition(filepath.Join(root, ".entire", "tmp", "poolside", files[0].Name()))
	if err != nil || position != len(tests) {
		t.Fatalf("position=%d err=%v", position, err)
	}
}

func TestParseHookRejectsMalformedPayload(t *testing.T) {
	if event, err := New().ParseHook("Stop", []byte("{")); err == nil || event != nil {
		t.Fatalf("expected malformed payload error, event=%#v err=%v", event, err)
	}
}

func TestInstallHooksPreservesUserConfigurationAndUninstallsOnlyOwnedEntries(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ENTIRE_REPO_ROOT", root)
	path := filepath.Join(root, poolsideSettingsFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	initial := "name: project\nhooks:\n  SessionStart:\n    - command: echo custom\n  Stop:\n    - command: entire hooks poolside Stop\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	agent := New()
	count, err := agent.InstallHooks(false, false)
	if err != nil || count != len(poolsideHookNames)-1 {
		t.Fatalf("install count=%d err=%v", count, err)
	}
	count, err = agent.InstallHooks(false, false)
	if err != nil || count != 0 || !agent.AreHooksInstalled() {
		t.Fatalf("second install count=%d err=%v installed=%v", count, err, agent.AreHooksInstalled())
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "echo custom") || !strings.Contains(string(data), "name: project") {
		t.Fatalf("user configuration was not preserved: %s", data)
	}
	if err := agent.UninstallHooks(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "echo custom") || strings.Contains(string(data), "entire hooks poolside") {
		t.Fatalf("uninstall changed the wrong settings: %s", data)
	}
}

func TestInstallHooksRejectsMalformedSettingsAndForceIsNotDestructive(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ENTIRE_REPO_ROOT", root)
	path := filepath.Join(root, poolsideSettingsFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := []byte("hooks: [not-a-map]\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New().InstallHooks(false, true); err == nil {
		t.Fatal("expected malformed settings error")
	}
	data, _ := os.ReadFile(path)
	if string(data) != string(contents) {
		t.Fatalf("malformed settings were overwritten: %s", data)
	}
}
