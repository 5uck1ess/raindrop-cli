package collectionsCmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/collections"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

var CollectionsCmd = &cobra.Command{
	Use:     "collections",
	Aliases: []string{"col"},
	Short:   "List, create, move, rename, and delete collections",
}

var listFlags struct {
	tree bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all collections (flat or as a tree)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		cols, err := collections.ListAll(c)
		if err != nil {
			u.PrintFatal("list collections", err)
		}
		if len(cols) == 0 {
			u.PrintInfo("no collections")
			return
		}

		if u.GlobalForAIFlag {
			fmt.Println("id\tparent_id\tcount\ttitle")
			for _, col := range cols {
				fmt.Printf("%d\t%d\t%d\t%s\n", col.ID, col.ParentID, col.Count, col.Title)
			}
			return
		}

		if listFlags.tree {
			printTree(cols)
			return
		}

		rows := make([][]string, 0, len(cols))
		for _, col := range cols {
			rows = append(rows, []string{
				fmt.Sprintf("%d", col.ID),
				fmt.Sprintf("%d", col.ParentID),
				fmt.Sprintf("%d", col.Count),
				col.Title,
			})
		}
		u.PrintTable([]string{"ID", "PARENT", "COUNT", "TITLE"}, rows)
	},
}

func printTree(cols []collections.Collection) {
	byParent := map[int][]collections.Collection{}
	for _, col := range cols {
		byParent[col.ParentID] = append(byParent[col.ParentID], col)
	}
	for _, kids := range byParent {
		sort.Slice(kids, func(i, j int) bool { return kids[i].Title < kids[j].Title })
	}
	var walk func(parentID, depth int)
	walk = func(parentID, depth int) {
		for _, col := range byParent[parentID] {
			fmt.Printf("%s%s (id=%d, count=%d)\n", strings.Repeat("  ", depth), col.Title, col.ID, col.Count)
			walk(col.ID, depth+1)
		}
	}
	walk(0, 0)
}

var createFlags struct {
	title  string
	parent int
	color  string
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new collection (prints new ID)",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		col, err := collections.Create(c, createFlags.title, createFlags.parent, createFlags.color)
		if err != nil {
			u.PrintFatal("create collection", err)
		}
		if u.GlobalForAIFlag {
			fmt.Println(col.ID)
			return
		}
		u.PrintSuccess(fmt.Sprintf("created %q (id=%d)", col.Title, col.ID))
		fmt.Println(col.ID)
	},
}

var moveFlags struct {
	id     int
	parent string
}

var moveCmd = &cobra.Command{
	Use:   "move",
	Short: "Reparent a collection (--parent root promotes to top-level)",
	Run: func(cmd *cobra.Command, args []string) {
		var parentID int
		if moveFlags.parent == "root" {
			parentID = 0
		} else {
			if _, err := fmt.Sscanf(moveFlags.parent, "%d", &parentID); err != nil {
				u.PrintFatal("invalid --parent (need numeric ID or 'root')", err)
			}
		}
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		if err := collections.Reparent(c, moveFlags.id, parentID); err != nil {
			u.PrintFatal("move collection", err)
		}
		u.PrintSuccess(fmt.Sprintf("moved %d → parent=%s", moveFlags.id, moveFlags.parent))
	},
}

var renameFlags struct {
	id int
	to string
}

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename a collection",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		if err := collections.Rename(c, renameFlags.id, renameFlags.to); err != nil {
			u.PrintFatal("rename collection", err)
		}
		u.PrintSuccess(fmt.Sprintf("renamed %d → %q", renameFlags.id, renameFlags.to))
	},
}

var deleteFlags struct {
	id    int
	force bool
	empty bool
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a collection; items are moved to Trash",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}

		if deleteFlags.empty {
			cols, err := collections.ListAll(c)
			if err != nil {
				u.PrintFatal("list collections", err)
			}
			var removed int
			for _, col := range cols {
				if col.Count != 0 {
					continue
				}
				if err := collections.Delete(c, col.ID); err != nil {
					u.PrintWarn(fmt.Sprintf("delete %d (%q)", col.ID, col.Title), err)
					continue
				}
				removed++
				u.PrintInfo(fmt.Sprintf("deleted %d (%q)", col.ID, col.Title))
			}
			u.PrintSuccess(fmt.Sprintf("pruned %d empty collection(s)", removed))
			return
		}

		if deleteFlags.id == 0 {
			u.PrintFatal("need --id or --empty", nil)
		}

		if !deleteFlags.force {
			cols, err := collections.ListAll(c)
			if err != nil {
				u.PrintFatal("list collections", err)
			}
			for _, col := range cols {
				if col.ID == deleteFlags.id && col.Count > 0 {
					u.PrintFatal(fmt.Sprintf("collection %d has %d item(s); pass --force to delete (items go to Trash)", col.ID, col.Count), nil)
				}
			}
		}
		if err := collections.Delete(c, deleteFlags.id); err != nil {
			u.PrintFatal("delete collection", err)
		}
		u.PrintSuccess(fmt.Sprintf("deleted collection %d (items moved to Trash)", deleteFlags.id))
	},
}

func init() {
	CollectionsCmd.AddCommand(listCmd, createCmd, moveCmd, renameCmd, deleteCmd)

	listCmd.Flags().BoolVar(&listFlags.tree, "tree", false, "Indented tree view")

	createCmd.Flags().StringVar(&createFlags.title, "title", "", "Collection title")
	createCmd.Flags().IntVar(&createFlags.parent, "parent", 0, "Parent collection ID (0=root)")
	createCmd.Flags().StringVar(&createFlags.color, "color", "", "Hex color, e.g. #ff0000")
	_ = createCmd.MarkFlagRequired("title")

	moveCmd.Flags().IntVar(&moveFlags.id, "id", 0, "Collection ID to move")
	moveCmd.Flags().StringVar(&moveFlags.parent, "parent", "", "Target parent ID, or 'root' to promote")
	_ = moveCmd.MarkFlagRequired("id")
	_ = moveCmd.MarkFlagRequired("parent")

	renameCmd.Flags().IntVar(&renameFlags.id, "id", 0, "Collection ID to rename")
	renameCmd.Flags().StringVar(&renameFlags.to, "to", "", "New title")
	_ = renameCmd.MarkFlagRequired("id")
	_ = renameCmd.MarkFlagRequired("to")

	deleteCmd.Flags().IntVar(&deleteFlags.id, "id", 0, "Collection ID to delete")
	deleteCmd.Flags().BoolVar(&deleteFlags.force, "force", false, "Delete even if non-empty (items → Trash)")
	deleteCmd.Flags().BoolVar(&deleteFlags.empty, "empty", false, "Prune all zero-count collections")
}
