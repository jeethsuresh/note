package cmd

import (
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.8labs.io/jsuresh/note/internal/auth"
	"code.8labs.io/jsuresh/note/internal/paths"
	"code.8labs.io/jsuresh/note/sync"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	loginServer   string
	loginUser     string
	loginPassword string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Configure server identity, register, and verify signing",
	Long: `Register with --password (admin) on first use, or omit --password to only verify keys and reachability.

Writes server and user to ~/.note.yaml (or --config).`,
	RunE: runLogin,
}

func runLogin(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(loginServer) == "" || strings.TrimSpace(loginUser) == "" {
		return fmt.Errorf("--server and --user are required")
	}
	notesDir, err := paths.NotesDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		return err
	}
	priv, err := auth.EnsureKeyPair(notesDir)
	if err != nil {
		return err
	}
	pub := priv.Public().(ed25519.PublicKey)
	if loginPassword != "" {
		if err := sync.Register(strings.TrimRight(loginServer, "/"), loginUser, loginPassword, pub); err != nil {
			return err
		}
	}
	cl := &sync.Client{
		BaseURL: strings.TrimRight(loginServer, "/"),
		User:    loginUser,
		Priv:    priv,
	}
	if err := cl.Whoami(); err != nil {
		return err
	}
	if err := writeLoginConfig(strings.TrimRight(loginServer, "/"), loginUser); err != nil {
		return err
	}
	fmt.Println("login: saved server and user to config")
	return nil
}

func writeLoginConfig(server, user string) error {
	var cfgPath string
	if cfgFile != "" {
		cfgPath = cfgFile
	} else {
		home, err := homedir.Dir()
		if err != nil {
			return err
		}
		cfgPath = filepath.Join(home, ".note.yaml")
	}
	viper.SetConfigFile(cfgPath)
	viper.SetConfigType("yaml")
	_ = viper.ReadInConfig()
	viper.Set("server", server)
	viper.Set("user", user)
	if err := viper.WriteConfig(); err != nil {
		return viper.WriteConfigAs(cfgPath)
	}
	return nil
}

func init() {
	loginCmd.Flags().StringVar(&loginServer, "server", "", "note server base URL (e.g. http://127.0.0.1:8080)")
	loginCmd.Flags().StringVar(&loginUser, "user", "", "username on the server")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "admin password for POST /v1/register (omit after registration)")
	rootCmd.AddCommand(loginCmd)
}
