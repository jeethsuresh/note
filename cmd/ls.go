package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/gobuffalo/envy"
	"github.com/spf13/cobra"
)

// lsCmd represents the ls command
var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		runcmd := exec.Command("bash", "-c", "ls "+envy.Get("HOME", "")+"/notes/** | awk -F'/' '{print $NF}'")
		runcmd.Stdout = os.Stdout
		runcmd.Stderr = os.Stderr
		runcmd.Stdin = os.Stdin
		if err := runcmd.Run(); err != nil {
			if err, ok := err.(*exec.Error); ok {
				if err.Err == exec.ErrNotFound {
					fmt.Println("unable to launch ls for directory 'notes'")
					return
				}
			}
			return
		}
		return
	},
}

func init() {
	rootCmd.AddCommand(lsCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// lsCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// lsCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
