package cmd

import (
	"fmt"
	"strings"

	"code.8labs.io/jsuresh/note/search"
	"github.com/spf13/cobra"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search",
	Short: "Searches the sqlite db",
	Long: `
		Usage: 
		note search <term>
		note search --grep <regex>
	.`,
	Run: func(cmd *cobra.Command, args []string) {
		foundMap, _ := search.SearchForString(strings.Join(args, " "))
		for key := range foundMap {
			fmt.Println("**** " + key + " ****")
			for _, v := range foundMap[key] {
				fmt.Println(v)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(searchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// searchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// searchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
