package schema

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xbpk3t/docs-alfred/pkg/output"
)

func TestBuildSchema_Basic(t *testing.T) {
	root := &cobra.Command{Use: "mycli", Short: "A test CLI"}
	sub := &cobra.Command{Use: "sub", Short: "A subcommand"}
	root.AddCommand(sub)

	s := BuildSchema(root)

	assert.Equal(t, "mycli", s.Use)
	assert.Equal(t, "A test CLI", s.Short)
	require.Len(t, s.Commands, 1)
	assert.Equal(t, "sub", s.Commands[0].Use)
}

func TestBuildSchema_Flags(t *testing.T) {
	root := &cobra.Command{Use: "mycli"}
	var name string
	var verbose bool
	root.Flags().StringVar(&name, "name", "default", "Your name")
	root.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	s := BuildSchema(root)

	require.Len(t, s.Flags, 2)

	nameFlag := s.Flags[0]
	assert.Equal(t, "name", nameFlag.Name)
	assert.Equal(t, "string", nameFlag.Type)
	assert.Equal(t, "default", nameFlag.Default)
	assert.Equal(t, "Your name", nameFlag.Description)

	verboseFlag := s.Flags[1]
	assert.Equal(t, "verbose", verboseFlag.Name)
	assert.Equal(t, "v", verboseFlag.Shorthand)
	assert.Equal(t, "bool", verboseFlag.Type)
	assert.Equal(t, "false", verboseFlag.Default)
}

func TestBuildSchema_HiddenSkipped(t *testing.T) {
	root := &cobra.Command{Use: "mycli"}
	visible := &cobra.Command{Use: "visible", Short: "Visible"}
	hidden := &cobra.Command{Use: "hidden", Short: "Hidden", Hidden: true}
	root.AddCommand(visible)
	root.AddCommand(hidden)

	s := BuildSchema(root)

	require.Len(t, s.Commands, 1)
	assert.Equal(t, "visible", s.Commands[0].Use)
}

func TestBuildSchema_NestedCommands(t *testing.T) {
	root := &cobra.Command{Use: "mycli"}
	parent := &cobra.Command{Use: "parent", Short: "Parent"}
	child := &cobra.Command{Use: "child", Short: "Child"}
	parent.AddCommand(child)
	root.AddCommand(parent)

	s := BuildSchema(root)

	require.Len(t, s.Commands, 1)
	require.Len(t, s.Commands[0].Commands, 1)
	assert.Equal(t, "child", s.Commands[0].Commands[0].Use)
}

func TestBuildSchema_HiddenFlag(t *testing.T) {
	root := &cobra.Command{Use: "mycli"}
	root.Flags().String("secret", "", "A secret flag")
	_ = root.Flags().MarkHidden("secret")

	s := BuildSchema(root)

	require.Len(t, s.Flags, 1)
	assert.True(t, s.Flags[0].Hidden)
}

func TestSchemaCmd_JSONOutput(t *testing.T) {
	root := &cobra.Command{Use: "mycli", Short: "A test CLI"}
	root.Flags().String("name", "default", "Your name")

	var format string
	output.FormatFlag(root, &format, output.FormatText, []string{output.FormatText, output.FormatJSON}, "format")
	root.AddCommand(SchemaCmd(root))

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"schema"})
	require.NoError(t, root.Execute())

	var s CommandSchema
	require.NoError(t, json.Unmarshal(buf.Bytes(), &s))
	assert.Equal(t, "mycli", s.Use)

	var nameFlag *FlagSchema
	for i := range s.Flags {
		if s.Flags[i].Name == "name" {
			nameFlag = &s.Flags[i]
			break
		}
	}
	require.NotNil(t, nameFlag, "expected 'name' flag in schema")
	assert.Equal(t, "string", nameFlag.Type)
	assert.Equal(t, "default", nameFlag.Default)
	assert.Equal(t, "Your name", nameFlag.Description)
}

func TestSchemaCmd_OutputIsJSON(t *testing.T) {
	root := &cobra.Command{Use: "mycli"}
	sub := &cobra.Command{Use: "sub", Short: "A sub"}
	root.AddCommand(sub)
	root.AddCommand(SchemaCmd(root))

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"schema"})
	require.NoError(t, root.Execute())

	var s CommandSchema
	require.NoError(t, json.Unmarshal(buf.Bytes(), &s))

	var subCmd *CommandSchema
	for i := range s.Commands {
		if s.Commands[i].Use == "sub" {
			subCmd = &s.Commands[i]
			break
		}
	}
	require.NotNil(t, subCmd, "expected 'sub' command in schema")
	assert.Equal(t, "A sub", subCmd.Short)
}

func TestSchemaCmd_CompletionSkipped(t *testing.T) {
	root := &cobra.Command{Use: "mycli"}
	root.AddCommand(SchemaCmd(root))

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"schema"})
	require.NoError(t, root.Execute())

	var s CommandSchema
	require.NoError(t, json.Unmarshal(buf.Bytes(), &s))

	for _, cmd := range s.Commands {
		assert.NotEqual(t, "completion", cmd.Use, "completion command should be skipped")
	}
}
