package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gptcode/internal/config"
)

var cavemanCmd = &cobra.Command{
	Use:   "caveman [level]",
	Short: "Configure or check Caveman Mode (token compression)",
	Long: `Gets or sets the Caveman Mode level for token compression.
Supported levels: off, lite, full, ultra, wenyan-lite, wenyan-full, wenyan-ultra.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			// Get current mode
			val, err := config.GetConfig("defaults.caveman_mode")
			if err != nil || val == "<nil>" || val == "" {
				val = "off"
			}
			fmt.Printf("Caveman Mode is currently: %s\n", val)
			return nil
		}

		level := strings.ToLower(args[0])
		if level == "on" {
			level = "full"
		}

		if level != "off" && level != "lite" && level != "full" && level != "ultra" &&
			level != "wenyan-lite" && level != "wenyan-full" && level != "wenyan-ultra" {
			return fmt.Errorf("invalid level. Choose one of: off, lite, full, ultra, wenyan-lite, wenyan-full, wenyan-ultra")
		}

		err := config.SetConfig("defaults.caveman_mode", level)
		if err != nil {
			return err
		}

		fmt.Printf("Caveman Mode set to: %s\n", level)
		return nil
	},
}
