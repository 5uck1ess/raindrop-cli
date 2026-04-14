package cmd

import (
	"fmt"
	"os"
	"time"

	collectionsCmd "github.com/5uck1ess/raindrop-cli/cmd/collections-cmd"
	doctorCmd "github.com/5uck1ess/raindrop-cli/cmd/doctor-cmd"
	raindropsCmd "github.com/5uck1ess/raindrop-cli/cmd/raindrops-cmd"
	tagsCmd "github.com/5uck1ess/raindrop-cli/cmd/tags-cmd"
	toolsCmd "github.com/5uck1ess/raindrop-cli/cmd/tools-cmd"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var AppVersion = "dev-build"
var debugFlag bool
var forAIFlag bool

var rootCmd = &cobra.Command{
	Use:     "raindrop",
	Short:   "CLI for Raindrop.io bookmark management",
	Version: AppVersion,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setupLogs() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.DateTime,
		NoColor:    false,
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debugFlag {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		u.GlobalDebugFlag = true
	}
	if forAIFlag {
		zerolog.SetGlobalLevel(zerolog.Disabled)
		u.GlobalForAIFlag = true
	}
}

func init() {
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().BoolVar(&debugFlag, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&forAIFlag, "for-ai", false, "AI-friendly output (plain text / markdown tables)")
	rootCmd.MarkFlagsMutuallyExclusive("debug", "for-ai")
	cobra.OnInitialize(setupLogs)

	rootCmd.AddCommand(raindropsCmd.RaindropsCmd)
	rootCmd.AddCommand(collectionsCmd.CollectionsCmd)
	rootCmd.AddCommand(tagsCmd.TagsCmd)
	rootCmd.AddCommand(toolsCmd.ToolsCmd)
	doctorCmd.SetVersion(AppVersion)
	rootCmd.AddCommand(doctorCmd.DoctorCmd)
}
