package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/cmd/devtools/internal"
	"github.com/xbpk3t/docs-alfred/pkg/wf"
)

// toolAction defines an available action for a tool type.
type toolAction struct {
	Title    string
	Subtitle string
	Name     string
}

// toolRunner executes a tool action on input.
type toolRunner func(action, input string) (string, error)

// toolDef is the single registration for list + run.
// Adding a tool: append here only — all stages read this table.
type toolDef struct {
	Run     toolRunner
	Type    string
	Actions []toolAction
}

// toolsIndex is the master list of all registered tools.
var toolsIndex = []toolDef{
	{
		Type: "base64",
		Actions: []toolAction{
			{Title: "encode", Subtitle: "Base64 编码", Name: "encode"},
			{Title: "decode", Subtitle: "Base64 解码", Name: "decode"},
		},
		Run: runBase64,
	},
}

// queryStage is the nested if depth implied by the current query.
type queryStage int

const (
	stageTools queryStage = iota
	stageActions
	stageInput
	stageRun
)

// parsedQuery is the single-SF state machine input.
// Tokens: [tool] [action] [input...]; input keeps internal spaces.
type parsedQuery struct {
	Tool         *toolDef
	Action       *toolAction
	ToolFilter   string
	ActionFilter string
	Input        string
	Stage        queryStage
}

// Execute creates and runs the root command.
func Execute() {
	err := newRootCmd().Execute()
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "devtools [query...]",
		Short: "开发者工具箱（单 Script Filter + query 状态机）",
		Long: `Alfred 单 Script Filter：用 query 做 if → if → execute。

  (空)                         列出 tool（valid:false + autocomplete）
  base64                       列出 encode/decode
  base64 encode                提示输入
  base64 encode hello world    执行并 valid:true 输出结果（交给 Clipboard）

中间层 valid:false：回车/Tab 把 autocomplete 写回同一 SF 的 query。
最终层 valid:true：arg=结果，连到 Clipboard（只复制、不粘贴）。`,
		SilenceUsage: true,
		Args:         cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printItems(handleQuery(strings.Join(args, " ")))
		},
	}

	root.SetHelpCommand(&cobra.Command{Hidden: true})
	return root
}

func printItems(items []wf.AlfredItem) error {
	output, err := (&wf.AlfredFormatter{}).Format(items)
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(output + "\n")
	return err
}

// handleQuery is the single-SF router (tool → action → input → run).
func handleQuery(query string) []wf.AlfredItem {
	p := parseQuery(query)
	switch p.Stage {
	case stageTools:
		return listTools(p.ToolFilter)
	case stageActions:
		return listActions(p.Tool, p.ActionFilter)
	case stageInput:
		return promptInput(p.Tool, p.Action)
	case stageRun:
		return runTool(p.Tool, p.Action, p.Input)
	default:
		return listTools("")
	}
}

// parseQuery implements:
//
//	""              → tools
//	"b" / "base"    → tools (filter)
//	"base64"        → actions
//	"base64 en"     → actions (filter)
//	"base64 encode" → input prompt
//	"base64 encode hello world" → run (input keeps spaces)
func parseQuery(query string) parsedQuery {
	q := strings.TrimSpace(query)
	if q == "" {
		return parsedQuery{Stage: stageTools}
	}

	toolToken, rest := cutFirst(q)
	tool, exactTool := matchTool(toolToken)
	if tool == nil || !exactTool {
		return parsedQuery{ToolFilter: toolToken, Stage: stageTools}
	}

	rest = strings.TrimSpace(rest)
	if rest == "" {
		return parsedQuery{Tool: tool, Stage: stageActions}
	}

	actionToken, rest2 := cutFirst(rest)
	action, exactAction := matchAction(tool, actionToken)
	if action == nil || !exactAction {
		return parsedQuery{
			Tool:         tool,
			ActionFilter: actionToken,
			Stage:        stageActions,
		}
	}

	input := strings.TrimSpace(rest2)
	if input == "" {
		return parsedQuery{Tool: tool, Action: action, Stage: stageInput}
	}
	return parsedQuery{Tool: tool, Action: action, Input: input, Stage: stageRun}
}

func cutFirst(s string) (first, rest string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimSpace(s[i+1:])
}

func matchTool(token string) (*toolDef, bool) {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return nil, false
	}
	var prefix *toolDef
	prefixCount := 0
	for i := range toolsIndex {
		t := &toolsIndex[i]
		name := strings.ToLower(t.Type)
		if name == token {
			return t, true
		}
		if strings.HasPrefix(name, token) {
			prefix = t
			prefixCount++
		}
	}
	// Unique prefix is not "exact" — stay on tools stage and filter.
	if prefixCount == 1 {
		return prefix, false
	}
	return nil, false
}

func matchAction(tool *toolDef, token string) (*toolAction, bool) {
	token = strings.ToLower(strings.TrimSpace(token))
	if tool == nil || token == "" {
		return nil, false
	}
	var prefix *toolAction
	prefixCount := 0
	for i := range tool.Actions {
		a := &tool.Actions[i]
		name := strings.ToLower(a.Name)
		title := strings.ToLower(a.Title)
		if name == token || title == token {
			return a, true
		}
		// base64 shorthands
		if tool.Type == "base64" {
			if token == "e" && name == "encode" {
				return a, true
			}
			if token == "d" && name == "decode" {
				return a, true
			}
		}
		if strings.HasPrefix(name, token) || strings.HasPrefix(title, token) {
			prefix = a
			prefixCount++
		}
	}
	if prefixCount == 1 {
		return prefix, false
	}
	return nil, false
}

func listTools(filter string) []wf.AlfredItem {
	filter = strings.ToLower(strings.TrimSpace(filter))
	items := make([]wf.AlfredItem, 0, len(toolsIndex))
	for _, t := range toolsIndex {
		if filter != "" && !strings.Contains(strings.ToLower(t.Type), filter) {
			continue
		}
		// valid:false + autocomplete → 回车把 query 扩成 "base64 "，仍在同一 SF。
		items = append(items, wf.AlfredItem{
			Title:        t.Type,
			Subtitle:     "选择工具类型",
			Autocomplete: t.Type + " ",
			Match:        t.Type,
			Valid:        false,
		})
	}
	if len(items) == 0 {
		return []wf.AlfredItem{{
			Title:    fmt.Sprintf("未知工具: %s", filter),
			Subtitle: "可选: " + knownToolTypes(),
			Valid:    false,
		}}
	}
	return items
}

func listActions(tool *toolDef, filter string) []wf.AlfredItem {
	if tool == nil {
		return []wf.AlfredItem{{
			Title:    "未知工具",
			Subtitle: "可选: " + knownToolTypes(),
			Valid:    false,
		}}
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	items := make([]wf.AlfredItem, 0, len(tool.Actions))
	for _, a := range tool.Actions {
		hay := strings.ToLower(a.Name + " " + a.Title + " " + a.Subtitle)
		if filter != "" && !strings.Contains(hay, filter) && !strings.HasPrefix(strings.ToLower(a.Name), filter) {
			continue
		}
		// Title 只有 encode/decode（列表干净）；路径在 autocomplete。
		items = append(items, wf.AlfredItem{
			Title:        a.Title,
			Subtitle:     a.Subtitle,
			Autocomplete: tool.Type + " " + a.Name + " ",
			Match:        a.Name + " " + a.Title + " " + a.Subtitle,
			Valid:        false,
		})
	}
	if len(items) == 0 {
		return []wf.AlfredItem{{
			Title:    fmt.Sprintf("未知操作: %s", filter),
			Subtitle: fmt.Sprintf("%s · 可选 action 见上一级", tool.Type),
			Valid:    false,
		}}
	}
	return items
}

func promptInput(tool *toolDef, action *toolAction) []wf.AlfredItem {
	actionName := ""
	if action != nil {
		actionName = action.Name
	}
	typeName := ""
	if tool != nil {
		typeName = tool.Type
	}
	return []wf.AlfredItem{{
		Title:    fmt.Sprintf("输入要 %s 的内容...", actionName),
		Subtitle: fmt.Sprintf("%s · %s", typeName, actionName),
		// Keep query at "tool action " so further typing appends input.
		Autocomplete: typeName + " " + actionName + " ",
		Valid:        false,
	}}
}

func runTool(tool *toolDef, action *toolAction, input string) []wf.AlfredItem {
	if tool == nil {
		return []wf.AlfredItem{{
			Title: "不支持的工具",
			Valid: false,
		}}
	}
	if tool.Run == nil {
		return []wf.AlfredItem{{
			Title: fmt.Sprintf("工具未注册执行器: %s", tool.Type),
			Valid: false,
		}}
	}
	if action == nil {
		return []wf.AlfredItem{{
			Title: "未知操作",
			Valid: false,
		}}
	}

	result, err := tool.Run(action.Name, input)
	if err != nil {
		return []wf.AlfredItem{{
			Title: fmt.Sprintf("❌ %v", err),
			Valid: false,
		}}
	}
	return []wf.AlfredItem{{
		Title:    result,
		Subtitle: "⏎ 复制结果",
		Arg:      result,
		Valid:    true,
	}}
}

func runBase64(action, input string) (string, error) {
	switch action {
	case "encode", "e":
		return internal.EncodeBase64(input), nil
	case "decode", "d":
		return internal.DecodeBase64(input)
	default:
		return "", fmt.Errorf("未知操作: %s (可用: encode, decode)", action)
	}
}

func knownToolTypes() string {
	names := make([]string, 0, len(toolsIndex))
	for _, t := range toolsIndex {
		names = append(names, t.Type)
	}
	if len(names) == 0 {
		return "(none)"
	}
	return strings.Join(names, ", ")
}
