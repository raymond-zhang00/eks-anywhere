/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"github.com/spf13/cobra"
)

// experimentCmd represents the experiment command
var expCmd = &cobra.Command{
	Use:   "exp",
	Short: "Experimental commands",
	Long:  "Use eksctl anywhere experimental commands",
}

func init() {
	rootCmd.AddCommand(expCmd)
}
