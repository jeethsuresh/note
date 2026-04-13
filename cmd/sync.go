package cmd

import (
	"fmt"

	"code.8labs.io/jsuresh/note/internal/auth"
	"code.8labs.io/jsuresh/note/internal/paths"
	"code.8labs.io/jsuresh/note/sync"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var syncTruth string

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync notes with a server",
	Long: `
		Usage: note sync [--truth merge|server|client|lastwrite]

Requires ~/.note.yaml server and user from "note login".
`,
	RunE: runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	server := viper.GetString("server")
	user := viper.GetString("user")
	if server == "" || user == "" {
		return fmt.Errorf("missing server or user in config; run: note login --server <url> --user <name>")
	}
	truth, err := sync.ParseTruth(syncTruth)
	if err != nil {
		return err
	}
	notesDir, err := paths.NotesDir()
	if err != nil {
		return err
	}
	privPath, _ := paths.KeyPaths(notesDir)
	priv, err := auth.LoadPrivateKey(privPath)
	if err != nil {
		return fmt.Errorf("load private key: %w (run note login)", err)
	}
	cl := &sync.Client{BaseURL: server, User: user, Priv: priv}
	if err := sync.Run(cl, sync.Options{NotesDir: notesDir, Truth: truth}); err != nil {
		return err
	}
	fmt.Println("sync: complete")
	return nil
}

func init() {
	syncCmd.Flags().StringVar(&syncTruth, "truth", "merge", "conflict resolution: merge (diff3), server, client, lastwrite")
	rootCmd.AddCommand(syncCmd)
}
