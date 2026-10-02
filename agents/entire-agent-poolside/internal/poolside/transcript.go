package poolside

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/entireio/external-agents/agents/entire-agent-poolside/internal/protocol"
)

type hookRecord struct {
	Event           string          `json:"event"`
	SessionID       string          `json:"session_id"`
	Timestamp       string          `json:"timestamp"`
	CWD             string          `json:"cwd,omitempty"`
	Prompt          string          `json:"prompt,omitempty"`
	ToolName        string          `json:"tool_name,omitempty"`
	ToolInput       json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse    json.RawMessage `json:"tool_response,omitempty"`
	ResponseMessage string          `json:"response_message,omitempty"`
}

func (a *Agent) sessionRef(input *protocol.HookInputJSON) string {
	if input != nil && strings.TrimSpace(input.SessionRef) != "" {
		return input.SessionRef
	}
	id := a.GetSessionID(input)
	dir, _ := a.GetSessionDir(protocol.RepoRoot())
	return a.ResolveSessionFile(dir, id)
}
func (a *Agent) ReadSession(input *protocol.HookInputJSON) (protocol.AgentSessionJSON, error) {
	ref := a.sessionRef(input)
	if err := validateSessionRef(ref); err != nil {
		return protocol.AgentSessionJSON{}, err
	}
	data, e := os.ReadFile(ref)
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return protocol.AgentSessionJSON{}, e
	}
	if errors.Is(e, os.ErrNotExist) {
		data = nil
	}
	files, _, _ := a.ExtractModifiedFiles(ref, 0)
	return protocol.AgentSessionJSON{SessionID: a.GetSessionID(input), AgentName: AgentName, RepoPath: protocol.RepoRoot(), SessionRef: ref, NativeData: data, ModifiedFiles: files, NewFiles: []string{}, DeletedFiles: []string{}}, nil
}
func (a *Agent) WriteSession(s protocol.AgentSessionJSON) error {
	if strings.TrimSpace(s.SessionRef) == "" {
		return errors.New("session_ref is required")
	}
	if err := validateSessionRef(s.SessionRef); err != nil {
		return err
	}
	if len(s.NativeData) == 0 {
		return errors.New("refusing to write empty Poolside transcript")
	}
	if e := os.MkdirAll(filepath.Dir(s.SessionRef), 0700); e != nil {
		return e
	}
	return os.WriteFile(s.SessionRef, s.NativeData, 0600)
}

func validateSessionRef(ref string) error {
	root, err := filepath.Abs(filepath.Join(protocol.RepoRoot(), ".entire", "tmp", "poolside"))
	if err != nil {
		return err
	}
	path, err := filepath.Abs(ref)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("session_ref must be inside %s", root)
	}
	return nil
}
func (a *Agent) ReadTranscript(ref string) ([]byte, error) { return os.ReadFile(ref) }
func (a *Agent) ChunkTranscript(b []byte, n int) ([][]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("max-size must be positive")
	}
	if len(b) == 0 {
		return [][]byte{{}}, nil
	}
	var out [][]byte
	for i := 0; i < len(b); i += n {
		j := i + n
		if j > len(b) {
			j = len(b)
		}
		out = append(out, append([]byte(nil), b[i:j]...))
	}
	return out, nil
}
func (a *Agent) ReassembleTranscript(c [][]byte) ([]byte, error) { return bytes.Join(c, nil), nil }
func (a *Agent) PrepareTranscript(ref string) error {
	if _, e := os.Stat(ref); e != nil {
		return e
	}
	return nil
}
func readRecords(path string) ([]hookRecord, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	var out []hookRecord
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 64<<20)
	for s.Scan() {
		var r hookRecord
		if e := json.Unmarshal(s.Bytes(), &r); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, s.Err()
}
func (a *Agent) GetTranscriptPosition(path string) (int, error) {
	r, e := readRecords(path)
	if errors.Is(e, os.ErrNotExist) {
		return 0, nil
	}
	return len(r), e
}
func (a *Agent) ExtractModifiedFiles(path string, offset int) ([]string, int, error) {
	r, e := readRecords(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil, 0, nil
	}
	if e != nil {
		return nil, 0, e
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(r) {
		offset = len(r)
	}
	seen := map[string]bool{}
	for _, x := range r[offset:] {
		if !isFileMutationTool(x.ToolName) {
			continue
		}
		var v any
		if json.Unmarshal(x.ToolInput, &v) == nil {
			collectPaths(v, seen)
		}
		if json.Unmarshal(x.ToolResponse, &v) == nil {
			collectPaths(v, seen)
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, len(r), nil
}
func (a *Agent) ExtractPrompts(path string, offset int) ([]string, error) {
	r, e := readRecords(path)
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(r) {
		offset = len(r)
	}
	var out []string
	for _, x := range r[offset:] {
		if x.Event == "UserPromptSubmit" && x.Prompt != "" {
			out = append(out, x.Prompt)
		}
	}
	return out, nil
}
func (a *Agent) ExtractSummary(path string) (string, bool, error) {
	r, e := readRecords(path)
	if errors.Is(e, os.ErrNotExist) {
		return "", false, nil
	}
	if e != nil {
		return "", false, e
	}
	for i := len(r) - 1; i >= 0; i-- {
		if r[i].ResponseMessage != "" {
			return r[i].ResponseMessage, true, nil
		}
	}
	return "", false, nil
}
func collectPaths(v any, seen map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, v := range x {
			lk := strings.ToLower(k)
			if lk == "path" || lk == "filepath" || lk == "file_path" || lk == "filename" || lk == "absolute_path" || lk == "relative_path" {
				if s, ok := v.(string); ok && s != "" {
					seen[s] = true
				}
			}
			collectPaths(v, seen)
		}
	case []any:
		for _, v := range x {
			collectPaths(v, seen)
		}
	}
}

func isFileMutationTool(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return false
	}
	for _, marker := range []string{"write", "edit", "patch", "replace", "move", "rename", "delete", "remove"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}
