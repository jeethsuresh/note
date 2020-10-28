package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gobuffalo/envy"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "note",
	Short: "Indexed notes synced to the cloud",
	Long: `
		Usage: 
		note sync: syncs files 
		note <topic> views a note on a specific topic
		note edit 
	`,
	Args: cobra.ArbitraryArgs,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("note default called")
		if len(args) != 1 {
			rootHelp()
			fmt.Println(len(args))
			return
		}
		runcmd := exec.Command("less", envy.Get("HOME", "")+"/notes/"+args[0]+".txt")
		runcmd.Stdout = os.Stdout
		runcmd.Stderr = os.Stderr
		runcmd.Stdin = os.Stdin
		if err := runcmd.Run(); err != nil {
			if err, ok := err.(*exec.Error); ok {
				if err.Err == exec.ErrNotFound {
					fmt.Printf("unable to launch less for file: %q\n", strings.Join(args, " "))
					return
				}
			}
			return
		}
		return
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.note.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".note" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".note")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func rootHelp() {
	fmt.Println(`
		Requires the name of an existing note. 

		note ls - lists all notes 
		note edit - edits notes
	`)
}
