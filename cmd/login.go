package cmd

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.8labs.io/jsuresh/note/internal/auth"
	"code.8labs.io/jsuresh/note/internal/paths"
	"code.8labs.io/jsuresh/note/sync"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"
)

var (
	loginServer   string
	loginUser     string
	loginPassword string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Configure server identity, register, and verify signing",
	Long: `Register with --password (admin) on first use to create the account or add this device's key,
or omit --password to only verify keys and reachability.

Writes server, user, and identity keys (user_private_key / user_public_key) to ~/.note.yaml or --config — safe to copy across your machines.
Each machine keeps its own device signing key only under ~/notes/note_device_ed25519 (generated here when missing).

The server stores identity.pub plus each device line in authorized_devices; HTTP requests are signed with the device key.`,
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
	userPriv, devicePriv, err := resolveKeyPairs(notesDir, true)
	if err != nil {
		return err
	}
	if err := auth.WriteUserKeyPair(notesDir, userPriv); err != nil {
		return err
	}
	if err := auth.WriteDeviceKeyPair(notesDir, devicePriv); err != nil {
		return err
	}
	userPub := userPriv.Public().(ed25519.PublicKey)
	devicePub := devicePriv.Public().(ed25519.PublicKey)
	if loginPassword != "" {
		err := sync.RegisterUser(strings.TrimRight(loginServer, "/"), loginUser, loginPassword, userPub, devicePub)
		if errors.Is(err, sync.ErrUserAlreadyExists) {
			err = sync.RegisterDevice(strings.TrimRight(loginServer, "/"), loginUser, loginPassword, devicePub)
		}
		if err != nil {
			return err
		}
	}
	cl := &sync.Client{
		BaseURL:    strings.TrimRight(loginServer, "/"),
		User:       loginUser,
		DevicePriv: devicePriv,
	}
	if err := cl.Whoami(); err != nil {
		return err
	}
	if err := writeLoginConfig(strings.TrimRight(loginServer, "/"), loginUser, userPriv); err != nil {
		return err
	}
	fmt.Println("login: saved server, user, and identity keys to config (device key stays local under ~/notes)")
	return nil
}

func writeLoginConfig(server, user string, userPriv ed25519.PrivateKey) error {
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

	data := map[string]interface{}{}
	if b, err := ioutil.ReadFile(cfgPath); err == nil {
		_ = yaml.Unmarshal(b, &data)
	}

	userPub := userPriv.Public().(ed25519.PublicKey)
	data["server"] = server
	data["user"] = user
	data["user_private_key"] = auth.PrivateKeyBase64(userPriv)
	data["user_public_key"] = auth.PublicKeyBase64(userPub)
	delete(data, "private_key")
	delete(data, "public_key")
	delete(data, "device_private_key")
	delete(data, "device_public_key")

	out, err := yaml.Marshal(&data)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(cfgPath, out, 0600)
}

func init() {
	loginCmd.Flags().StringVar(&loginServer, "server", "", "note server base URL (e.g. http://127.0.0.1:8080)")
	loginCmd.Flags().StringVar(&loginUser, "user", "", "username on the server")
	loginCmd.Flags().StringVar(&loginPassword, "password", "", "admin password for POST /v1/register and /v1/register-device")
	rootCmd.AddCommand(loginCmd)
}
