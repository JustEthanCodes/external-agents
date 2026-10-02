package poolside

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/entireio/external-agents/agents/entire-agent-poolside/internal/protocol"
)

const (
	AgentName = "poolside"
	AgentType = "Poolside"
)

type Agent struct{ LookPath func(string) (string, error) }

func New() *Agent { return &Agent{LookPath: exec.LookPath} }

func (a *Agent) Info() protocol.InfoResponse {
	return protocol.InfoResponse{ProtocolVersion: protocol.ProtocolVersion, Name: AgentName, Type: AgentType, Description: "Poolside coding agent integration for Entire", IsPreview: true, ProtectedDirs: []string{".poolside"}, HookNames: []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop", "PreCompact"}, Capabilities: protocol.DeclaredCapabilities{Hooks: true, TranscriptAnalyzer: true, UsesTerminal: true}}
}
func (a *Agent) Detect() protocol.DetectResponse {
	lp := a.LookPath
	if lp == nil {
		lp = exec.LookPath
	}
	_, e := lp("pool")
	return protocol.DetectResponse{Present: e == nil}
}
func (a *Agent) GetSessionID(in *protocol.HookInputJSON) string {
	if in != nil {
		return strings.TrimSpace(in.SessionID)
	}
	return ""
}
func (a *Agent) GetSessionDir(repo string) (string, error) {
	if strings.TrimSpace(repo) == "" {
		repo = protocol.RepoRoot()
	}
	return filepath.Join(repo, ".entire", "tmp", "poolside"), nil
}
func (a *Agent) ResolveSessionFile(dir, id string) string {
	return filepath.Join(dir, safeFilename(id)+".jsonl")
}
func (a *Agent) FormatResumeCommand(id string) string {
	if strings.TrimSpace(id) == "" {
		return "pool"
	}
	return "pool -r " + shellQuote(id)
}
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if strings.IndexFunc(s, func(r rune) bool {
		return !(r == '-' || r == '_' || r == '.' || r == ':' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
func safeFilename(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' || r == ':' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	name := strings.Trim(b.String(), "._")
	if name == "" {
		return "unknown"
	}
	return name
}
