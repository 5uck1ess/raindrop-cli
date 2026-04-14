package toolsCmd

import (
	"fmt"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/raindrops"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

var collectionFlag int
var dryRunFlag bool

var ToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Cleanup helpers: dedup, broken links",
}

var dedupCmd = &cobra.Command{
	Use:   "dedup",
	Short: "Find and optionally remove duplicate bookmarks (by link)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}

		seen := map[string][]int{}
		page := 0
		for {
			items, _, err := raindrops.List(c, collectionFlag, "", page, 50)
			if err != nil {
				u.PrintFatal("scan", err)
			}
			if len(items) == 0 {
				break
			}
			for _, r := range items {
				seen[r.Link] = append(seen[r.Link], r.ID)
			}
			if len(items) < 50 {
				break
			}
			page++
		}

		var dupIDs []int
		for _, ids := range seen {
			if len(ids) > 1 {
				dupIDs = append(dupIDs, ids[1:]...)
			}
		}
		if len(dupIDs) == 0 {
			u.PrintSuccess("no duplicates found")
			return
		}
		u.PrintInfo(fmt.Sprintf("found %d duplicate bookmarks", len(dupIDs)))
		if dryRunFlag {
			u.PrintInfo("[dry-run] not deleting")
			return
		}
		for i := 0; i < len(dupIDs); i += 100 {
			end := i + 100
			if end > len(dupIDs) {
				end = len(dupIDs)
			}
			if err := raindrops.DeleteMany(c, collectionFlag, dupIDs[i:end]); err != nil {
				u.PrintFatal("delete batch", err)
			}
		}
		u.PrintSuccess(fmt.Sprintf("deleted %d duplicates", len(dupIDs)))
	},
}

var brokenCmd = &cobra.Command{
	Use:   "broken",
	Short: "List bookmarks Raindrop flagged as broken",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		items, total, err := raindrops.List(c, collectionFlag, "broken", 0, 50)
		if err != nil {
			u.PrintFatal("search broken", err)
		}
		if len(items) == 0 {
			u.PrintSuccess("no broken links")
			return
		}
		rows := make([][]string, 0, len(items))
		for _, r := range items {
			rows = append(rows, []string{fmt.Sprintf("%d", r.ID), r.Title, r.Link})
		}
		u.PrintTable([]string{"ID", "TITLE", "LINK"}, rows)
		u.PrintInfo(fmt.Sprintf("showing %d of %d", len(items), total))
	},
}

func init() {
	ToolsCmd.PersistentFlags().IntVarP(&collectionFlag, "collection", "c", 0, "Collection ID (0 = all)")
	ToolsCmd.PersistentFlags().BoolVar(&dryRunFlag, "dry-run", false, "Preview without writing")
	ToolsCmd.AddCommand(dedupCmd)
	ToolsCmd.AddCommand(brokenCmd)
}
