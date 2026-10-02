package poolside

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseHook(t *testing.T){
	root:=t.TempDir();t.Setenv("ENTIRE_REPO_ROOT",root)
	a:=New()
	input:=map[string]any{"session_id":"sess-1","timestamp":"2026-10-02T13:00:00Z","cwd":root,"prompt":"hello"}
	b,_:=json.Marshal(input)
	ev,e:=a.ParseHook("UserPromptSubmit",b);if e!=nil{t.Fatal(e)}
	if ev==nil||ev.Type!=2||ev.SessionID!="sess-1"||ev.Prompt!="hello"{t.Fatalf("unexpected event: %#v",ev)}
	ref:=filepath.Join(root,".entire","tmp","poolside","sess-1.jsonl")
	if _,e:=os.Stat(ref);e!=nil{t.Fatalf("transcript not written: %v",e)}
}
func TestEmptyHookIsIgnored(t *testing.T){if ev,e:=New().ParseHook("UserPromptSubmit",nil);e!=nil||ev!=nil{t.Fatalf("got %#v %v",ev,e)}}
