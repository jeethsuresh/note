package cmd

import (
	"fmt"

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

Requires ~/.note.yaml (server, user, identity keys) and local ~/notes/note_device_ed25519 from "note login" on this machine.
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
	userPriv, devicePriv, err := resolveKeyPairs(notesDir, false)
	if err != nil {
		return fmt.Errorf("keys: %w", err)
	}
	if err := persistKeyFilesIfMissing(notesDir, userPriv, devicePriv); err != nil {
		return err
	}
	cl := &sync.Client{BaseURL: server, User: user, DevicePriv: devicePriv}
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
