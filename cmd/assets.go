package cmd

import (
	"github.com/spf13/cobra"
)

var assetsCmd = &cobra.Command{
	Use:   "assets",
	Short: "Manage files in Tier0 object storage",
	Long: `Upload, download, get URL, and delete files in Tier0 object storage.

Examples:
  tier0 assets upload ./report.csv --visibility private
  tier0 assets download --file-path workspace/.../report.csv -o ./report.csv
  tier0 assets url --file-path workspace/.../report.csv
  tier0 assets delete --file-path workspace/.../report.csv`,
}

func init() {
	assetsCmd.AddCommand(assetsUploadCmd)
	assetsCmd.AddCommand(assetsDownloadCmd)
	assetsCmd.AddCommand(assetsUrlCmd)
	assetsCmd.AddCommand(assetsDeleteCmd)
}
