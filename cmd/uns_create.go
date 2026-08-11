package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/FREEZONEX/Tier0-cli/internal/cmdutil"
	"github.com/FREEZONEX/Tier0-cli/internal/notice"
	"github.com/spf13/cobra"
)

var unsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create UNS namespace nodes",
	Long: "Create UNS namespace nodes from a path or a JSON file.\n\n" +
		"PATH RULE for topic (file) nodes:\n" +
		"  The segment immediately before the leaf must be a type folder: Metric, Action, or State.\n" +
		"  The topicType is derived from that segment automatically — nothing is inserted.\n\n" +
		"  Valid:   Plant/Line1/Metric/Temperature\n" +
		"  Valid:   Machine/Action/Start\n" +
		"  Invalid: Plant/Line1/Temperature  (no type folder before leaf)\n\n" +
		"Use --parent to prepend a common path prefix to --topic.\n" +
		"Use --file for batch or complex structures.\n\n" +
		"Examples:\n" +
		"  tier0 uns create --topic Plant/Line1/Metric/Temperature --type topic\n" +
		"  tier0 uns create --parent Factory1/Line1/Station1 --topic Metric/ProductionCount --type topic\n" +
		"  tier0 uns create --topic Plant/Line1 --type path --display-name 'Line 1'\n" +
		"  tier0 uns create --file namespace.json",

	RunE: runUnsCreate,
}

func init() {
	unsCreateCmd.Flags().StringP("topic", "t", "",
		"Topic path or leaf name (required if not using --file)")
	unsCreateCmd.Flags().String("parent", "",
		"Parent path prefix (optional, combined with --topic)")
	unsCreateCmd.Flags().String("type", "",
		"Node type: 'path' (folder) or 'topic' (data point)")
	unsCreateCmd.Flags().StringP("display-name", "d", "",
		"Display name")
	unsCreateCmd.Flags().String("description", "",
		"Description")
	unsCreateCmd.Flags().String("alias", "",
		"Alias")
	unsCreateCmd.Flags().StringP("file", "f", "",
		"Read namespace definition from JSON file ({\"namespace\":[]} or bare array)")
	unsCreateCmd.Flags().String("topic-type", "",
		"Deprecated: topic type is now derived from the path (Metric/Action/State folder before leaf)")
	unsCreateCmd.Flags().String("fields", "",
		"Schema fields JSON array (e.g. '[{\"name\":\"temp\",\"type\":\"float\"}]')")
	unsCreateCmd.Flags().String("fields-file", "",
		"Read schema fields JSON array from file")
	addDryRunFlag(unsCreateCmd)
}

func runUnsCreate(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	debug, _ := cmd.Flags().GetBool("debug")
	topic, _ := cmd.Flags().GetString("topic")
	parent, _ := cmd.Flags().GetString("parent")
	nodeType, _ := cmd.Flags().GetString("type")
	displayName, _ := cmd.Flags().GetString("display-name")
	description, _ := cmd.Flags().GetString("description")
	alias, _ := cmd.Flags().GetString("alias")
	file, _ := cmd.Flags().GetString("file")
	topicType, _ := cmd.Flags().GetString("topic-type")
	fields, _ := cmd.Flags().GetString("fields")
	fieldsFile, _ := cmd.Flags().GetString("fields-file")

	var namespace []any
	createdPath := ""

	errOut := cmd.ErrOrStderr()
	if file != "" {
		var conflicts []string
		for _, name := range []string{"topic", "parent", "type", "display-name", "description", "alias", "topic-type", "fields", "fields-file"} {
			if cmd.Flags().Changed(name) {
				conflicts = append(conflicts, "--"+name)
			}
		}
		if len(conflicts) > 0 {
			return invalidArgument(cmd, "--file", "--file cannot be combined with "+strings.Join(conflicts, ", "))
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			return fileIOError(cmd, "--file", "read namespace file", file, err)
		}
		namespace, err = parseNamespaceFile(raw)
		if err != nil {
			return invalidArgumentCause(cmd, "--file", "namespace file is invalid: "+err.Error(), err)
		}
		if err := validateNamespaceTree(namespace); err != nil {
			return invalidArgumentCause(cmd, "--file", "namespace file is invalid: "+err.Error(), err)
		}
	} else {
		if topic == "" || nodeType == "" {
			return invalidArgument(cmd, "--topic/--type", "--topic and --type are required (or use --file)")
		}
		if cmd.Flags().Changed("fields") && cmd.Flags().Changed("fields-file") {
			return invalidArgument(cmd, "--fields/--fields-file", "--fields and --fields-file are mutually exclusive")
		}
		if cmd.Flags().Changed("fields-file") {
			raw, err := os.ReadFile(fieldsFile)
			if err != nil {
				return fileIOError(cmd, "--fields-file", "read fields file", fieldsFile, err)
			}
			fields = string(raw)
		}
		var err error
		namespace, createdPath, err = buildNamespaceFromFlags(parent, topic, nodeType, topicType, displayName, description, alias, fields, errOut)
		if err != nil {
			return invalidArgumentCause(cmd, "--topic/--type", err.Error(), err)
		}
	}

	payload := map[string]any{"namespace": namespace}
	if handled, err := writeDryRun(cmd, "POST", "/openapi/v1/uns/create", payload); handled {
		return err
	}

	checker := notice.Start()
	body := cmdutil.JSONString(payload)
	resp, err := cmdutil.DoAPI(cmd.Context(), "/openapi/v1/uns/create", "POST", body, debug)
	if err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}
	if err := cmdutil.CheckOK(resp); err != nil {
		return cmdutil.HandleCommandError(cmd.ErrOrStderr(), err, jsonMode)
	}

	stdout := cmd.OutOrStdout()
	checker.Emit(resp, jsonMode, stdout, cmd.ErrOrStderr())
	if !jsonMode && createdPath != "" {
		fmt.Fprintf(stdout, "Topic created: %s\n", createdPath)
	} else if !jsonMode {
		fmt.Fprintln(stdout, "Namespace created.")
	}
	return nil
}
