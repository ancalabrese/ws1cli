package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout the currently authenticated account",
	RunE:  runLogout,
}

func init() {
	RootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	_, err := deleteConfig()
	if err != nil {
		return fmt.Errorf("logging out user: %w", err)
	}
	return nil
}
