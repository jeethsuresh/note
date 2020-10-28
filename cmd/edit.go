package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"code.8labs.io/jsuresh/note/analyze"
	"github.com/gobuffalo/envy"
	"github.com/spf13/cobra"
)

// editCmd represents the edit command
var editCmd = &cobra.Command{
	Use:   "edit",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Printf("bad edit call with args %+v\n", args)
		}
		runcmd := exec.Command("vi", envy.Get("HOME", "")+"/notes/"+args[0]+".txt")
		runcmd.Stdout = os.Stdout
		runcmd.Stderr = os.Stderr
		runcmd.Stdin = os.Stdin
		if err := runcmd.Run(); err != nil {
			if err, ok := err.(*exec.Error); ok {
				if err.Err == exec.ErrNotFound {
					fmt.Printf("unable to launch vi for file: %q\n", strings.Join(args, " "))
				}
			}
			return
		}

		analyze.AnalyzeFile(args[0])
		return
	},
}

func init() {
	rootCmd.AddCommand(editCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// editCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// editCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
