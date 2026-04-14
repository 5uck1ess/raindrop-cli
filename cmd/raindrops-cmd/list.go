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
	collection        int
	search            string
	page              int
	perPage           int
	includeCollection bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List bookmarks in a collection (auto-paginates by default)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}

		paged := cmd.Flags().Changed("page") || cmd.Flags().Changed("per-page")

		var items []raindrops.Raindrop
		var total int
		if paged {
			items, total, err = raindrops.List(c, listFlags.collection, listFlags.search, listFlags.page, listFlags.perPage)
			if err != nil {
				u.PrintFatal("list failed", err)
			}
		} else {
			items, err = raindrops.ListAll(c, listFlags.collection, listFlags.search)
			if err != nil {
				u.PrintFatal("list failed", err)
			}
			total = len(items)
		}

		if len(items) == 0 {
			u.PrintInfo("no bookmarks")
			return
		}

		headers := []string{"ID", "TITLE", "DOMAIN", "TAGS"}
		if listFlags.includeCollection {
			headers = []string{"ID", "COLLECTION", "TITLE", "DOMAIN", "TAGS"}
		}
		rows := make([][]string, 0, len(items))
		for _, r := range items {
			row := []string{fmt.Sprintf("%d", r.ID)}
			if listFlags.includeCollection {
				row = append(row, fmt.Sprintf("%d", r.CollectionID()))
			}
			row = append(row,
				truncate(r.Title, 60),
				r.Domain,
				strings.Join(r.Tags, ","),
			)
			rows = append(rows, row)
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
	listCmd.Flags().IntVar(&listFlags.page, "page", 0, "Page number (0-indexed); setting this disables auto-pagination")
	listCmd.Flags().IntVar(&listFlags.perPage, "per-page", 50, "Items per page (max 50); setting this disables auto-pagination")
	listCmd.Flags().BoolVar(&listFlags.includeCollection, "include-collection", false, "Include COLLECTION column in output")
}
