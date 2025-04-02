package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ec2cli",
	Short: "CLI tool to manage EC2 instances",
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.AddCommand(launchCmd)
	rootCmd.AddCommand(terminateCmd)
	rootCmd.AddCommand(sshCmd)
}
