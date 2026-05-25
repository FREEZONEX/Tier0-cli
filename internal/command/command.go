// Package command provides a declarative command builder for Cobra.
// It is a simplified version of the Lark CLI "Shortcut" pattern,
// stripped of OAuth scopes, identity, and other Tier0-unnecessary complexity.
package command

import (
	"strconv"

	"github.com/spf13/cobra"
)

// FlagType enumerates the supported flag types.
type FlagType string

const (
	FlagString      FlagType = "string"
	FlagBool        FlagType = "bool"
	FlagInt         FlagType = "int"
	FlagStringSlice FlagType = "stringSlice"
)

// Flag describes a CLI flag.
type Flag struct {
	Name      string
	Shorthand string
	Type      FlagType
	Default   string
	Desc      string
	Required  bool
}

// Cmd is a declarative command definition that builds a *cobra.Command.
type Cmd struct {
	Use   string
	Short string
	Long  string
	Flags []Flag
	Args  cobra.PositionalArgs
	RunE  func(cmd *cobra.Command, args []string) error
}

// Build converts the declarative Cmd into a *cobra.Command with flags registered.
func (c Cmd) Build() *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   c.Use,
		Short: c.Short,
		Long:  c.Long,
		Args:  c.Args,
		RunE:  c.RunE,
	}
	for _, f := range c.Flags {
		switch f.Type {
		case FlagBool:
			def := f.Default == "true"
			cobraCmd.Flags().BoolP(f.Name, f.Shorthand, def, f.Desc)
		case FlagInt:
			def := 0
			if f.Default != "" {
				def, _ = strconv.Atoi(f.Default)
			}
			cobraCmd.Flags().IntP(f.Name, f.Shorthand, def, f.Desc)
		case FlagStringSlice:
			cobraCmd.Flags().StringSliceP(f.Name, f.Shorthand, nil, f.Desc)
		default:
			cobraCmd.Flags().StringP(f.Name, f.Shorthand, f.Default, f.Desc)
		}
		if f.Required {
			cobraCmd.MarkFlagRequired(f.Name)
		}
	}
	return cobraCmd
}
