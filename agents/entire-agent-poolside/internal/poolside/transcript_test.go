package poolside

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/entireio/external-agents/agents/entire-agent-poolside/internal/protocol"
)

func TestTranscriptChunkRoundTrip(t *testing.T) {
	agent := New()
	content := []byte(strings.Repeat("record-", 1000))
	chunks, err := agent.ChunkTranscript(content, 37)
	if err != nil {
		t.Fatal(err)
	}
	reassembled, err := agent.ReassembleTranscript(chunks)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reassembled, content) {
		t.Fatal("transcript chunk round trip changed bytes")
	}
	if _, err := agent.ChunkTranscript(content, 0); err == nil {
		t.Fatal("expected invalid chunk size error")
	}
}

func TestTranscriptAnalyzersHonorOffsetsAndAvoidReadToolFalsePositives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	data := strings.Join([]string{
		`{"event":"SessionStart","session_id":"s","timestamp":"2026-10-02T00:00:00Z"}`,
		`{"event":"UserPromptSubmit","session_id":"s","timestamp":"2026-10-02T00:00:01Z","prompt":"first"}`,
		`{"event":"PreToolUse","session_id":"s","timestamp":"2026-10-02T00:00:02Z","tool_name":"read_file","tool_input":{"path":"not-modified.txt"}}`,
		`{"event":"PostToolUse","session_id":"s","timestamp":"2026-10-02T00:00:03Z","tool_name":"write_file","tool_input":{"file_path":"z.txt"},"tool_response":{"path":"a.txt"}}`,
		`{"event":"UserPromptSubmit","session_id":"s","timestamp":"2026-10-02T00:00:04Z","prompt":"second"}`,
		`{"event":"Stop","session_id":"s","timestamp":"2026-10-02T00:00:05Z","response_message":"done"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	agent := New()
	files, position, err := agent.ExtractModifiedFiles(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if position != 6 || !reflect.DeepEqual(files, []string{"a.txt", "z.txt"}) {
		t.Fatalf("files=%v position=%d", files, position)
	}
	files, position, err = agent.ExtractModifiedFiles(path, 3)
	if err != nil || position != 6 || !reflect.DeepEqual(files, []string{"a.txt", "z.txt"}) {
		t.Fatalf("offset files=%v position=%d err=%v", files, position, err)
	}
	prompts, err := agent.ExtractPrompts(path, 2)
	if err != nil || !reflect.DeepEqual(prompts, []string{"second"}) {
		t.Fatalf("prompts=%v err=%v", prompts, err)
	}
	summary, hasSummary, err := agent.ExtractSummary(path)
	if err != nil || !hasSummary || summary != "done" {
		t.Fatalf("summary=%q has=%v err=%v", summary, hasSummary, err)
	}
}

func TestTranscriptAnalyzersRejectMalformedAndSupportLargeRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	largePrompt := strings.Repeat("x", 17<<20)
	data := `{"event":"UserPromptSubmit","session_id":"s","prompt":"` + largePrompt + `"}` + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	prompts, err := New().ExtractPrompts(path, 0)
	if err != nil || len(prompts) != 1 || len(prompts[0]) != len(largePrompt) {
		t.Fatalf("large prompt length=%d err=%v", len(prompts[0]), err)
	}
	if err := os.WriteFile(path, []byte("{\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New().GetTranscriptPosition(path); err == nil {
		t.Fatal("expected malformed transcript error")
	}
}

func TestReadSessionDoesNotOverwriteAndMissingTranscriptIsEmpty(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ENTIRE_REPO_ROOT", root)
	session := protocolHookInput("missing")
	result, err := New().ReadSession(session)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.NativeData) != 0 || result.SessionID != "missing" {
		t.Fatalf("unexpected missing session result: %#v", result)
	}
	if err := New().WriteSession(result); err == nil {
		t.Fatal("expected empty session write to be rejected")
	}
}

func protocolHookInput(id string) *protocol.HookInputJSON {
	return &protocol.HookInputJSON{SessionID: id}
}
