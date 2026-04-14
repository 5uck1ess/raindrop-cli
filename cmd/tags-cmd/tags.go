package tagsCmd

import (
	"fmt"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/tags"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

var collectionFlag int
var dryRunFlag bool

var TagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List, merge, rename, and delete tags",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tags with counts",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		items, err := tags.List(c, collectionFlag)
		if err != nil {
			u.PrintFatal("list tags", err)
		}
		if len(items) == 0 {
			u.PrintInfo("no tags")
			return
		}
		rows := make([][]string, 0, len(items))
		for _, t := range items {
			rows = append(rows, []string{t.Name, fmt.Sprintf("%d", t.Count)})
		}
		u.PrintTable([]string{"TAG", "COUNT"}, rows)
	},
}

var mergeFlags struct {
	from []string
	to   string
}

var mergeCmd = &cobra.Command{
	Use:   "merge",
	Short: "Merge multiple tags into one",
	Run: func(cmd *cobra.Command, args []string) {
		if dryRunFlag {
			u.PrintInfo(fmt.Sprintf("[dry-run] would merge %v → %q", mergeFlags.from, mergeFlags.to))
			return
		}
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		if err := tags.Merge(c, collectionFlag, mergeFlags.from, mergeFlags.to); err != nil {
			u.PrintFatal("merge", err)
		}
		u.PrintSuccess(fmt.Sprintf("merged %v → %q", mergeFlags.from, mergeFlags.to))
	},
}

var renameFlags struct {
	from string
	to   string
}

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename one tag",
	Run: func(cmd *cobra.Command, args []string) {
		if dryRunFlag {
			u.PrintInfo(fmt.Sprintf("[dry-run] would rename %q → %q", renameFlags.from, renameFlags.to))
			return
		}
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		if err := tags.Merge(c, collectionFlag, []string{renameFlags.from}, renameFlags.to); err != nil {
			u.PrintFatal("rename", err)
		}
		u.PrintSuccess(fmt.Sprintf("renamed %q → %q", renameFlags.from, renameFlags.to))
	},
}

var deleteFlags struct {
	names []string
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete tags (untags all bookmarks)",
	Run: func(cmd *cobra.Command, args []string) {
		if dryRunFlag {
			u.PrintInfo(fmt.Sprintf("[dry-run] would delete %v", deleteFlags.names))
			return
		}
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		if err := tags.Delete(c, collectionFlag, deleteFlags.names); err != nil {
			u.PrintFatal("delete", err)
		}
		u.PrintSuccess(fmt.Sprintf("deleted %v", deleteFlags.names))
	},
}

func init() {
	TagsCmd.PersistentFlags().IntVarP(&collectionFlag, "collection", "c", 0, "Collection ID (0 = all)")
	TagsCmd.PersistentFlags().BoolVar(&dryRunFlag, "dry-run", false, "Preview without writing")

	TagsCmd.AddCommand(listCmd)
	TagsCmd.AddCommand(mergeCmd)
	TagsCmd.AddCommand(renameCmd)
	TagsCmd.AddCommand(deleteCmd)

	mergeCmd.Flags().StringSliceVar(&mergeFlags.from, "from", nil, "Tags to merge (comma-separated)")
	mergeCmd.Flags().StringVar(&mergeFlags.to, "to", "", "Target tag name")
	_ = mergeCmd.MarkFlagRequired("from")
	_ = mergeCmd.MarkFlagRequired("to")

	renameCmd.Flags().StringVar(&renameFlags.from, "from", "", "Existing tag name")
	renameCmd.Flags().StringVar(&renameFlags.to, "to", "", "New tag name")
	_ = renameCmd.MarkFlagRequired("from")
	_ = renameCmd.MarkFlagRequired("to")

	deleteCmd.Flags().StringSliceVar(&deleteFlags.names, "tag", nil, "Tags to delete (comma-separated)")
	_ = deleteCmd.MarkFlagRequired("tag")
}
