package cmd

import (
	"database/sql"
	"log"
	"os"

	"github.com/gobuffalo/envy"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {

		os.MkdirAll(envy.Get("HOME", "")+"/notes", 0755)
		os.Create(envy.Get("HOME", "") + "/notes/note.db")

		db, err := sql.Open("sqlite3", envy.Get("HOME", "")+"/notes/note.db")
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		sqlStmt := `
		create table tokens (token text not null, document text not null, count integer, PRIMARY KEY(token, document));
	`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Printf("%q: %s\n", err, sqlStmt)
			return
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	if reindex := initCmd.Flags().BoolP("index", "i", true, "Re-index existing notes in the folder"); reindex != nil {
		if *reindex {
			//TODO: get all files and run them through the analyzer
		}
	}
}
