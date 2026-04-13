package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"code.8labs.io/jsuresh/note/internal/ainotes"
	"code.8labs.io/jsuresh/note/internal/paths"
	"github.com/spf13/cobra"
)

var aiFromPath string

var aiCmd = &cobra.Command{
	Use:   "ai",
	Short: "Agent-oriented note operations (list, create, edit, delete, search, trash)",
}

var aiListCmd = &cobra.Command{
	Use:   "list",
	Short: "Print absolute paths of AI notes (`ai-*.txt` in the notes directory)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		ps, err := ainotes.ListAINotePaths(notesDir)
		if err != nil {
			return err
		}
		for _, p := range ps {
			fmt.Println(p)
		}
		return nil
	},
}

var aiCreateCmd = &cobra.Command{
	Use:   "create <slug>",
	Short: "Create `ai-<topic>.txt` (slug must start with `ai-`; empty unless --from is set)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		if err := ainotes.CreateNote(notesDir, args[0], aiFromPath); err != nil {
			return err
		}
		p, err := filepath.Abs(paths.NoteFile(notesDir, args[0]))
		if err != nil {
			return err
		}
		fmt.Println(p)
		return nil
	},
}

var aiEditCmd = &cobra.Command{
	Use:   "edit <slug> [replacement-file]",
	Short: "Slug must start with `ai-`: print note path, or replace from file and re-index",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		slug := args[0]
		if len(args) == 1 {
			p, err := ainotes.NotePath(notesDir, slug)
			if err != nil {
				return err
			}
			if _, err := os.Stat(p); err != nil {
				return err
			}
			fmt.Println(p)
			return nil
		}
		rep := args[1]
		if !filepath.IsAbs(rep) {
			if rep, err = filepath.Abs(rep); err != nil {
				return err
			}
		}
		if err := ainotes.ReplaceNote(notesDir, slug, rep); err != nil {
			return err
		}
		out, err := filepath.Abs(paths.NoteFile(notesDir, slug))
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

var aiDeleteCmd = &cobra.Command{
	Use:   "delete <slug>",
	Short: "Move an `ai-` note into trash and remove it from the search index",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		return ainotes.DeleteNote(notesDir, args[0])
	},
}

var aiSearchCmd = &cobra.Command{
	Use:   "search <term> [term...]",
	Short: "Search only `ai-*.txt` notes; list lines where every term appears (case-insensitive)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		matches, err := ainotes.SearchTerms(notesDir, args)
		if err != nil {
			return err
		}
		for _, m := range matches {
			fmt.Printf("%s:%d:%d:%s\n", m.Path, m.Line, m.Column, m.Text)
		}
		return nil
	},
}

var aiTrashCmd = &cobra.Command{
	Use:   "trash",
	Short: "Inspect or manage the local trash folder",
}

var aiTrashListCmd = &cobra.Command{
	Use:   "list",
	Short: "List trashed note files (basename one per line)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		names, err := ainotes.ListTrash(notesDir)
		if err != nil {
			return err
		}
		trashDir := paths.TrashDir(notesDir)
		for _, n := range names {
			fmt.Println(filepath.Join(trashDir, n))
		}
		return nil
	},
}

var aiTrashRestoreCmd = &cobra.Command{
	Use:   "restore <basename>",
	Short: "Restore a trashed file back into the notes directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		base := filepath.Base(args[0])
		restored, err := ainotes.RestoreTrash(notesDir, base)
		if err != nil {
			return err
		}
		fmt.Println(restored)
		return nil
	},
}

var aiTrashRmCmd = &cobra.Command{
	Use:   "rm <basename>",
	Short: "Permanently remove a file from trash",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		notesDir, err := paths.NotesDir()
		if err != nil {
			return err
		}
		return ainotes.PurgeTrash(notesDir, filepath.Base(args[0]))
	},
}

func init() {
	aiCreateCmd.Flags().StringVar(&aiFromPath, "from", "", "populate the new note from this file")
	aiCmd.AddCommand(aiListCmd, aiCreateCmd, aiEditCmd, aiDeleteCmd, aiSearchCmd, aiTrashCmd)
	aiTrashCmd.AddCommand(aiTrashListCmd, aiTrashRestoreCmd, aiTrashRmCmd)
	rootCmd.AddCommand(aiCmd)
}
