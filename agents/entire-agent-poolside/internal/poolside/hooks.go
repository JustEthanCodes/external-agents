package poolside

import(
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"github.com/entireio/external-agents/agents/entire-agent-poolside/internal/protocol"
)

type rawHook struct{SessionID string `json:"session_id"`;Timestamp string `json:"timestamp"`;CWD string `json:"cwd"`;Prompt string `json:"prompt"`;ToolName string `json:"tool_name"`;ToolInput json.RawMessage `json:"tool_input"`;ToolResponse json.RawMessage `json:"tool_response"`;InitialPrompt string `json:"initial_prompt"`;Reason string `json:"reason"`}
func(a *Agent)ParseHook(name string,input []byte)(*protocol.EventJSON,error){
	input=bytes.TrimSpace(input);if len(input)==0{return nil,nil};var raw rawHook;if e:=json.Unmarshal(input,&raw);e!=nil{return nil,fmt.Errorf("parse Poolside hook input: %w",e)};if raw.SessionID==""{return nil,nil};if raw.Timestamp==""{raw.Timestamp=time.Now().UTC().Format(time.RFC3339)}
	ref:=a.ResolveSessionFile(filepath.Join(protocol.RepoRoot(),".entire","tmp","poolside"),raw.SessionID)
	if e:=appendRecord(ref,name,raw);e!=nil{return nil,e}
	meta:=map[string]string{"agent":"poolside","hook_event_name":name};if raw.CWD!=""{meta["cwd"]=raw.CWD};if raw.Reason!=""{meta["reason"]=raw.Reason}
	switch name{
	case"SessionStart":return &protocol.EventJSON{Type:1,SessionID:raw.SessionID,SessionRef:ref,Timestamp:raw.Timestamp,Prompt:raw.InitialPrompt,Metadata:meta},nil
	case"UserPromptSubmit":return &protocol.EventJSON{Type:2,SessionID:raw.SessionID,SessionRef:ref,Timestamp:raw.Timestamp,Prompt:raw.Prompt,Metadata:meta},nil
	case"Stop":return &protocol.EventJSON{Type:3,SessionID:raw.SessionID,SessionRef:ref,Timestamp:raw.Timestamp,ResponseMessage:extractText(raw.ToolResponse),Metadata:meta},nil
	case"PreCompact":return &protocol.EventJSON{Type:4,SessionID:raw.SessionID,SessionRef:ref,Timestamp:raw.Timestamp,Metadata:meta},nil
	default:return nil,nil}
}
func extractText(b json.RawMessage)string{var s string;if json.Unmarshal(b,&s)==nil{return s};var v map[string]any;if json.Unmarshal(b,&v)==nil{for _,k:=range []string{"text","message","content","output"}{if s,ok:=v[k].(string);ok{return s}}};return ""}
func appendRecord(ref,name string,raw rawHook)error{if e:=os.MkdirAll(filepath.Dir(ref),0700);e!=nil{return e};rec:=map[string]any{"event":name,"session_id":raw.SessionID,"timestamp":raw.Timestamp,"cwd":raw.CWD,"prompt":raw.Prompt,"tool_name":raw.ToolName,"tool_input":raw.ToolInput,"tool_response":raw.ToolResponse,"response_message":extractText(raw.ToolResponse)};b,e:=json.Marshal(rec);if e!=nil{return e};f,e:=os.OpenFile(ref,os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600);if e!=nil{return e};defer f.Close();_,e=f.Write(append(b,'\n'));return e}
func(a *Agent)InstallHooks(_ bool,force bool)(int,error){
	path:=filepath.Join(protocol.RepoRoot(),".poolside","settings.yaml");if !force{if _,e:=os.Stat(path);e==nil{return 0,nil}}
	if e:=os.MkdirAll(filepath.Dir(path),0700);e!=nil{return 0,e}
	const marker="# entire-poolside: managed hooks\n"
	const body="hooks:\n  SessionStart:\n    - command: \"entire hooks poolside SessionStart\"\n  UserPromptSubmit:\n    - command: \"entire hooks poolside UserPromptSubmit\"\n  PreToolUse:\n    - command: \"entire hooks poolside PreToolUse\"\n  PostToolUse:\n    - command: \"entire hooks poolside PostToolUse\"\n  Stop:\n    - command: \"entire hooks poolside Stop\"\n  PreCompact:\n    - command: \"entire hooks poolside PreCompact\"\n"
	if _,e:=os.Stat(path);e==nil { if force { return 0, fmt.Errorf("refusing to overwrite existing %s; add the Entire hook entries manually or remove the file after backing it up", path) }; return 0,nil }; if e:=os.WriteFile(path,[]byte(marker+body),0600);e!=nil{return 0,e};return 6,nil
}
func(a *Agent)UninstallHooks()error{path:=filepath.Join(protocol.RepoRoot(),".poolside","settings.yaml");b,e:=os.ReadFile(path);if errors.Is(e,os.ErrNotExist){return nil};if e!=nil{return e};lines:=strings.Split(string(b),"\n");var out []string;skip:=false;for _,line:=range lines{if strings.HasPrefix(line,"# entire-poolside: managed hooks"){skip=true;continue};if skip&&line=="hooks:"{continue};if skip&&(strings.HasPrefix(line,"  ")||line==""){continue};skip=false;out=append(out,line)};return os.WriteFile(path,[]byte(strings.Join(out,"\n")),0600)}
func(a *Agent)AreHooksInstalled()bool{b,e:=os.ReadFile(filepath.Join(protocol.RepoRoot(),".poolside","settings.yaml"));return e==nil&&bytes.Contains(b,[]byte("# entire-poolside: managed hooks"))}
