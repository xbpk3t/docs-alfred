package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// CommandSchema represents a CLI command tree.
type CommandSchema struct {
	Use        string          `json:"use"`
	Short      string          `json:"short"`
	Long       string          `json:"long,omitempty"`
	Deprecated string          `json:"deprecated,omitempty"`
	Args       string          `json:"args,omitempty"`
	Aliases    []string        `json:"aliases,omitempty"`
	Flags      []FlagSchema    `json:"flags,omitempty"`
	Commands   []CommandSchema `json:"commands,omitempty"`
	Hidden     bool            `json:"hidden,omitempty"`
}

// FlagSchema represents a command flag.
type FlagSchema struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
}

// SchemaCmd returns a command that dumps the root command tree as JSON.
func SchemaCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Output command schema as JSON for agent introspection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			schema := BuildSchema(root)

			data, err := json.MarshalIndent(schema, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal schema: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))

			return err
		},
	}
}

// BuildSchema recursively converts a cobra.Command tree to CommandSchema.
func BuildSchema(cmd *cobra.Command) CommandSchema {
	s := CommandSchema{
		Use:        cmd.Use,
		Short:      cmd.Short,
		Long:       cmd.Long,
		Aliases:    cmd.Aliases,
		Deprecated: cmd.Deprecated,
		Hidden:     cmd.Hidden,
	}

	if cmd.Args != nil {
		s.Args = argsDescription(cmd)
	}

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		s.Flags = append(s.Flags, FlagSchema{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        flagType(f),
			Default:     f.DefValue,
			Description: f.Usage,
			Required:    false, // cobra doesn't expose this at flag level easily
			Hidden:      f.Hidden,
		})
	})

	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "completion" {
			continue
		}

		s.Commands = append(s.Commands, BuildSchema(child))
	}

	return s
}

func flagType(f *pflag.Flag) string {
	if f.Value == nil {
		return "string"
	}

	t := f.Value.Type()
	switch t {
	case "bool":
		return "bool"
	case "int":
		return "int"
	case "stringArray", "stringSlice":
		return "stringArray"
	case "intSlice":
		return "intArray"
	default:
		return t
	}
}

func argsDescription(cmd *cobra.Command) string {
	if cmd.Args == nil {
		return ""
	}

	// Call with zero args to discover the constraint from the error message.
	if err := cmd.Args(cmd, []string{}); err != nil {
		msg := err.Error()
		if idx := strings.Index(msg, "accepts"); idx >= 0 {
			return msg[idx:]
		}

		return msg
	}

	// No error with zero args → accepts any args or exact(0).
	if err := cmd.Args(cmd, []string{"x"}); err != nil {
		return "exact(0)"
	}

	return "any"
}
