package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var configCmdFlags struct {
	server  string
	tenant  string
	version VersionFlag
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Show or update persistent CLI configuration.",
	Long: `Show or update persistent CLI configuration.

Run without flags to show the current configuration.
Pass flags to update specific fields — unspecified fields are left unchanged.

Example first-time setup:
  ws1 config --server as1831.awmdm.com --version 2506
  ws1 login --client-id <id> --secret <secret>`,
	RunE: runConfigCmd,
}

func init() {
	RootCmd.AddCommand(configCmd)
	configCmd.Flags().StringVar(&configCmdFlags.server, "server", "", "WS1 UEM server hostname (e.g. as1831.awmdm.com)")
	configCmd.Flags().StringVar(&configCmdFlags.tenant, "tenant", "", "aw-tenant-code header value")
	configCmd.Flags().Var(&configCmdFlags.version, "version", "WS1 UEM server version (one of: "+configCmdFlags.version.Type()+")")
}

func runConfigCmd(cmd *cobra.Command, args []string) error {
	cfg, _ := loadConfig()

	anySet := cmd.Flags().Changed("server") ||
		cmd.Flags().Changed("tenant") ||
		cmd.Flags().Changed("version")

	if !anySet {
		printConfig(cmd, cfg)
		return nil
	}

	if cmd.Flags().Changed("server") {
		cfg.Server = configCmdFlags.server
	}
	if cmd.Flags().Changed("tenant") {
		cfg.Tenant = configCmdFlags.tenant
	}
	if cmd.Flags().Changed("version") {
		ver := configCmdFlags.version.Value
		if !ver.IsSupported() {
			return fmt.Errorf("version %s is not yet supported", ver)
		}
		cfg.Version = ver
	}

	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	printConfig(cmd, cfg)
	return nil
}

func printConfig(cmd *cobra.Command, cfg config) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Current configuration:")
	fmt.Fprintf(out, "  server:  %s\n", orUnset(cfg.Server))
	fmt.Fprintf(out, "  version: %s\n", orUnset(cfg.Version.String()))
	fmt.Fprintf(out, "  tenant:  %s\n", orUnset(cfg.Tenant))
	if cfg.Token != "" {
		fmt.Fprintf(out, "  token:   %s…\n", cfg.Token[:min(16, len(cfg.Token))])
	} else {
		fmt.Fprintf(out, "  token:   %s\n", orUnset(""))
	}
	if cfg.ClientID != "" {
		fmt.Fprintf(out, "  client:  %s / %s\n", cfg.ClientID, strings.Repeat("*", 8))
	}
}

func orUnset(s string) string {
	if s == "" || s == "unknown" {
		return "(not set)"
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
