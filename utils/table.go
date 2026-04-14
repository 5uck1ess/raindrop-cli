package utils

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.ANSIColor(15)).Padding(0, 1)
	cellStyle   = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(7)).Padding(0, 1)
	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))
)

func PrintTable(headers []string, rows [][]string) {
	if GlobalForAIFlag {
		printTSV(headers, rows)
		return
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})

	PrintGeneric(t.Render())
}

func printTSV(headers []string, rows [][]string) {
	if len(headers) == 0 {
		return
	}
	fmt.Println(strings.Join(lower(headers), "\t"))
	for _, row := range rows {
		fmt.Println(strings.Join(sanitizeTab(row), "\t"))
	}
}

func lower(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = strings.ToLower(c)
	}
	return out
}

func sanitizeTab(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		c = strings.ReplaceAll(c, "\t", " ")
		c = strings.ReplaceAll(c, "\n", " ")
		out[i] = c
	}
	return out
}
