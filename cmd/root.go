// Package cmd provides the commands run by the ovplusplus command.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// name of the config file (without extension)
	cfgName = "ovplusplus"
	cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ovplusplus",
	Short: "Merge an IRRDB database with RPKI OV data into a single file.",
	// TODO(dotwaffle): Long description, and usage
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if viper.GetBool("debug") {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("rootCmd.Execute()")
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		fmt.Sprintf("config file (default is $HOME/.config/%s/%s.yaml)", cfgName, cfgName))

	rootCmd.PersistentFlags().BoolP("debug", "d", false, "output debug logging messages")
	if err := viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		log.Fatal().Err(err).Msg("viper.BindPFlag(): debug")
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName(cfgName)

		// Search config in local directory.
		viper.AddConfigPath(".")

		// Search config in home directory (preferably in XDG config).
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			log.Fatal().Err(err).Msg("os.UserConfigDir()")
		}
		viper.AddConfigPath(filepath.Join(cfgDir, cfgName)) // subdir
		viper.AddConfigPath(cfgDir)
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatal().Err(err).Msg("os.UserHomeDir()")
		}
		viper.AddConfigPath(homeDir)

		// Search config in other places.
		viper.AddConfigPath(filepath.Join("/etc", cfgName))
		viper.AddConfigPath("/etc")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Info().Str("file", viper.ConfigFileUsed()).Msg("using stored config")
	}
}

// writeConfigCmd writes the currently set configuration out.
var writeConfigCmd = &cobra.Command{
	Use:   "write-config",
	Short: "Writes the currently set configuration out.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch len(args) {
		case 0:
			viper.WriteConfigAs(fmt.Sprintf("./%s.yaml", cfgName))
		case 1:
			viper.WriteConfigAs(args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(writeConfigCmd)
}
