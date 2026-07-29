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
// Adding a tool: append here only — L1/L2/L3 all read this table.
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
		Use:   "devtools",
		Short: "开发者工具箱（Alfred 链式 Script Filter）",
		Long: `Alfred 链式多级选择：

  tools              第一级：列出 tool（选中后进入下一级 SF）
  actions <tool>     第二级：列出 encode/decode
  run <tool> <act> [input...]  第三级：提示输入或输出结果

业务仍是嵌套：tool → action → input → 执行。
Alfred 图：SF → Arg/Vars → SF → Arg/Vars → SF → Clipboard。
Script 正文必须用环境变量 $tool / $action（不要写 {var:tool}）。`,
		SilenceUsage: true,
	}

	root.AddCommand(newToolsCmd())
	root.AddCommand(newActionsCmd())
	root.AddCommand(newRunCmd())
	root.SetHelpCommand(&cobra.Command{Hidden: true})

	return root
}

func newToolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tools",
		Short: "List tool types for Alfred level 1",
		RunE: func(cmd *cobra.Command, args []string) error {
			return printItems(listToolTypes())
		},
	}
}

func newActionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "actions <tool>",
		Short: "List actions for a tool (Alfred level 2)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return printItems(listActionsForType(args[0]))
		},
	}
}

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <tool> <action> [input...]",
		Short: "Prompt for input or execute (Alfred level 3)",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := strings.Join(args[2:], " ")
			return printItems(executeAction(args[0], args[1], input))
		},
	}
}

func printItems(items []wf.AlfredItem) error {
	output, err := (&wf.AlfredFormatter{}).Format(items)
	if err != nil {
		return err
	}
	_, err = os.Stdout.WriteString(output + "\n")
	return err
}

// listToolTypes is Alfred level 1.
// valid=true + arg=tool so selection leaves this SF and chains to the next.
func listToolTypes() []wf.AlfredItem {
	items := make([]wf.AlfredItem, 0, len(toolsIndex))
	for _, t := range toolsIndex {
		items = append(items, wf.AlfredItem{
			Title:    t.Type,
			Subtitle: "选择工具类型",
			Arg:      t.Type,
			Match:    t.Type,
			Valid:    true,
		})
	}
	return items
}

// listActionsForType is Alfred level 2 (after Arg/Vars stored tool).
// Title is short; arg is only the action name for the next hop.
func listActionsForType(typeName string) []wf.AlfredItem {
	tool := findTool(typeName)
	if tool == nil {
		return []wf.AlfredItem{{
			Title:    fmt.Sprintf("未知工具: %s", typeName),
			Subtitle: "可选: " + knownToolTypes(),
			Valid:    false,
		}}
	}

	items := make([]wf.AlfredItem, 0, len(tool.Actions))
	for _, a := range tool.Actions {
		items = append(items, wf.AlfredItem{
			Title:    a.Title,
			Subtitle: a.Subtitle,
			Arg:      a.Name,
			Match:    a.Name + " " + a.Title + " " + a.Subtitle,
			Valid:    true,
		})
	}
	return items
}

// executeAction is Alfred level 3 — always via toolDef.Run from the registry.
func executeAction(typeName, actionName, input string) []wf.AlfredItem {
	tool := findTool(typeName)
	if tool == nil {
		return []wf.AlfredItem{{
			Title: fmt.Sprintf("不支持的工具: %s", typeName),
			Valid: false,
		}}
	}
	if tool.Run == nil {
		return []wf.AlfredItem{{
			Title: fmt.Sprintf("工具未注册执行器: %s", typeName),
			Valid: false,
		}}
	}

	if input == "" {
		return []wf.AlfredItem{{
			Title:    fmt.Sprintf("输入要 %s 的内容...", actionName),
			Subtitle: fmt.Sprintf("%s · %s", typeName, actionName),
			Valid:    false,
		}}
	}

	if !actionAllowed(tool, actionName) {
		return []wf.AlfredItem{{
			Title: fmt.Sprintf("未知操作: %s", actionName),
			Valid: false,
		}}
	}

	result, err := tool.Run(actionName, input)
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

func actionAllowed(tool *toolDef, name string) bool {
	if tool == nil {
		return false
	}
	for _, a := range tool.Actions {
		if a.Name == name || a.Title == name {
			return true
		}
	}
	// base64 shorthands accepted by runBase64
	if tool.Type == "base64" && (name == "e" || name == "d") {
		return true
	}
	return false
}

func findTool(name string) *toolDef {
	for i := range toolsIndex {
		if toolsIndex[i].Type == name {
			return &toolsIndex[i]
		}
	}
	return nil
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
