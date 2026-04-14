package raindropsCmd

import (
	"fmt"
	"strings"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/raindrops"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

var listFlags struct {
	collection int
	search     string
	page       int
	perPage    int
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List bookmarks in a collection",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		items, total, err := raindrops.List(c, listFlags.collection, listFlags.search, listFlags.page, listFlags.perPage)
		if err != nil {
			u.PrintFatal("list failed", err)
		}
		if len(items) == 0 {
			u.PrintInfo("no bookmarks")
			return
		}

		headers := []string{"ID", "TITLE", "DOMAIN", "TAGS"}
		rows := make([][]string, 0, len(items))
		for _, r := range items {
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				truncate(r.Title, 60),
				r.Domain,
				strings.Join(r.Tags, ","),
			})
		}
		u.PrintTable(headers, rows)
		u.PrintInfo(fmt.Sprintf("showing %d of %d", len(items), total))
	},
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func init() {
	RaindropsCmd.AddCommand(listCmd)
	listCmd.Flags().IntVarP(&listFlags.collection, "collection", "c", 0, "Collection ID (0=all, -1=unsorted, -99=trash)")
	listCmd.Flags().StringVarP(&listFlags.search, "search", "s", "", "Raindrop search query")
	listCmd.Flags().IntVar(&listFlags.page, "page", 0, "Page number (0-indexed)")
	listCmd.Flags().IntVar(&listFlags.perPage, "per-page", 50, "Items per page (max 50)")
}
