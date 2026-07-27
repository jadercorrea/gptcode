package main

import (
	"github.com/jadercorrea/gptcode/internal/auth"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with GPTCode Platform",
	Long: `Log in to your GPTCode account to access Pro/Team features.
This will open your browser to complete the authentication flow.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return auth.Login()
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from GPTCode Platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Just overwrite with empty/remove file
		_ = auth.SaveCredentials(&auth.Credentials{})
		return nil
	},
}
