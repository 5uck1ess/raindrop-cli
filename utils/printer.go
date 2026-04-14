package utils

import (
	"fmt"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/rs/zerolog/log"
)

var (
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(12))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(10))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(9))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(11))
)

func PrintInfo(msg string) {
	switch {
	case GlobalDebugFlag:
		log.Info().Str("package", "utils").Msg(msg)
	case GlobalForAIFlag:
		fmt.Println("[INFO] " + msg)
	default:
		fmt.Println(infoStyle.Render("→ " + msg))
	}
}

func PrintSuccess(msg string) {
	switch {
	case GlobalDebugFlag:
		log.Info().Str("package", "utils").Msg(msg)
	case GlobalForAIFlag:
		fmt.Println("[OK] " + msg)
	default:
		fmt.Println(successStyle.Render("✓ " + msg))
	}
}

func withErr(msg string, err error) string {
	if err == nil {
		return msg
	}
	return msg + ": " + err.Error()
}

func PrintWarn(msg string, err error) {
	switch {
	case GlobalDebugFlag:
		if err != nil {
			log.Warn().Err(err).Msg(msg)
		} else {
			log.Warn().Msg(msg)
		}
	case GlobalForAIFlag:
		fmt.Fprintln(os.Stderr, "[WARN] "+withErr(msg, err))
	default:
		fmt.Fprintln(os.Stderr, warnStyle.Render("! "+withErr(msg, err)))
	}
}

func PrintError(msg string, err error) {
	switch {
	case GlobalDebugFlag:
		if err != nil {
			log.Error().Err(err).Msg(msg)
		} else {
			log.Error().Msg(msg)
		}
	case GlobalForAIFlag:
		fmt.Fprintln(os.Stderr, "[ERROR] "+withErr(msg, err))
	default:
		fmt.Fprintln(os.Stderr, errorStyle.Render("✗ "+withErr(msg, err)))
	}
}

func PrintFatal(msg string, err error) {
	PrintError(msg, err)
	os.Exit(1)
}

func PrintGeneric(msg string) {
	fmt.Println(msg)
}
