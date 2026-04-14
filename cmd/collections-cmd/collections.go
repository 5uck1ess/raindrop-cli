package collectionsCmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
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

		if listFlags.tree {
			renderTree(cols, u.GlobalForAIFlag)
			return
		}

		if u.GlobalForAIFlag {
			fmt.Println("id\tparent_id\tcount\ttitle")
			for _, col := range cols {
				fmt.Printf("%d\t%d\t%d\t%s\n", col.ID, col.ParentID, col.Count, col.Title)
			}
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

// renderTree prints collections as an indented hierarchy. Roots sort by the
// Raindrop `sort` field descending; children sort by count descending.
// Collections whose parent_id points to a missing collection surface at the
// root tagged "[orphan]".
func renderTree(cols []collections.Collection, forAI bool) {
	exists := make(map[int]bool, len(cols))
	for _, col := range cols {
		exists[col.ID] = true
	}

	byParent := map[int][]collections.Collection{}
	orphan := map[int]bool{}
	for _, col := range cols {
		parent := col.ParentID
		if parent != 0 && !exists[parent] {
			orphan[col.ID] = true
			parent = 0
		}
		byParent[parent] = append(byParent[parent], col)
	}

	for pid, kids := range byParent {
		kids := kids
		if pid == 0 {
			sort.SliceStable(kids, func(i, j int) bool { return kids[i].Sort > kids[j].Sort })
		} else {
			sort.SliceStable(kids, func(i, j int) bool { return kids[i].Count > kids[j].Count })
		}
		byParent[pid] = kids
	}

	if forAI {
		fmt.Println("depth\tid\tparent_id\tcount\ttitle")
	} else {
		fmt.Printf("%-11s %5s   %s\n", "id", "count", "title")
	}

	var walk func(pid, depth int)
	walk = func(pid, depth int) {
		for _, col := range byParent[pid] {
			title := col.Title
			if orphan[col.ID] {
				title += " [orphan]"
			}
			if forAI {
				fmt.Printf("%d\t%d\t%d\t%d\t%s\n", depth, col.ID, col.ParentID, col.Count, title)
			} else {
				fmt.Printf("%-11d %5d   %s%s\n", col.ID, col.Count, strings.Repeat("  ", depth), title)
			}
			walk(col.ID, depth+1)
		}
	}
	walk(0, 0)
}

var createFlags struct {
	title  string
	parent int
	color  string
	quiet  bool
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
		if u.GlobalForAIFlag || createFlags.quiet {
			fmt.Println(col.ID)
			return
		}
		u.PrintSuccess(fmt.Sprintf("created %q (id=%d)", col.Title, col.ID))
		fmt.Println(col.ID)
	},
}

var moveFlags struct {
	id       int
	parent   string
	fromFile string
	progress bool
	dryRun   bool
}

var moveCmd = &cobra.Command{
	Use:   "move",
	Short: "Reparent one collection (--id) or many (--from-file TSV)",
	Long: `Modes:

  --id N --parent M|root                 reparent one collection
  --from-file plan.tsv                   TSV: <collection_id>\t<new_parent_id> (use 0 for root)

Collections API has no bulk endpoint; --from-file iterates row-by-row at
the client throttle (~100 req/min).`,
	Run: func(cmd *cobra.Command, args []string) {
		if moveFlags.fromFile != "" {
			runCollectionsMoveFromFile()
			return
		}
		var parentID int
		if moveFlags.parent == "root" {
			parentID = 0
		} else {
			if _, err := fmt.Sscanf(moveFlags.parent, "%d", &parentID); err != nil {
				u.PrintFatal("invalid --parent (need numeric ID or 'root')", err)
			}
		}
		if moveFlags.dryRun {
			u.PrintInfo(fmt.Sprintf("[dry-run] move collection %d → parent=%s", moveFlags.id, moveFlags.parent))
			return
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

type collectionMoveRow struct {
	id, newParent int
}

func runCollectionsMoveFromFile() {
	rows, err := parseCollectionMoveTSV(moveFlags.fromFile)
	if err != nil {
		u.PrintFatal("parse tsv", err)
	}
	if len(rows) == 0 {
		u.PrintInfo("no entries")
		return
	}
	if moveFlags.dryRun {
		for _, r := range rows {
			u.PrintInfo(fmt.Sprintf("[dry-run] collection %d → parent=%d", r.id, r.newParent))
		}
		u.PrintInfo(fmt.Sprintf("[dry-run] %d collection(s)", len(rows)))
		return
	}
	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}
	ok, fail := 0, 0
	for i, r := range rows {
		if err := collections.Reparent(c, r.id, r.newParent); err != nil {
			u.PrintWarn(fmt.Sprintf("id=%d → parent=%d", r.id, r.newParent), err)
			fail++
		} else {
			ok++
		}
		if moveFlags.progress {
			fmt.Fprintf(os.Stderr, "[%d/%d] moved collection %d → %d\n", i+1, len(rows), r.id, r.newParent)
		}
	}
	u.PrintSuccess(fmt.Sprintf("collection move: %d applied, %d failed", ok, fail))
}

func parseCollectionMoveTSV(path string) ([]collectionMoveRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	var out []collectionMoveRow
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimRight(sc.Text(), "\r\n")
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		parts := strings.SplitN(raw, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: expected <collection_id>\\t<new_parent_id>", line)
		}
		id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: bad id: %w", line, err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("line %d: bad new_parent_id: %w", line, err)
		}
		out = append(out, collectionMoveRow{id: id, newParent: pid})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
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
	id         int
	force      bool
	empty      bool
	leafOnly   bool
	excludeIDs []int
	dryRun     bool
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
			hasChildren := make(map[int]bool, len(cols))
			for _, col := range cols {
				if col.ParentID != 0 {
					hasChildren[col.ParentID] = true
				}
			}
			excluded := make(map[int]bool, len(deleteFlags.excludeIDs))
			for _, id := range deleteFlags.excludeIDs {
				excluded[id] = true
			}
			var targets []collections.Collection
			for _, col := range cols {
				if col.Count != 0 {
					continue
				}
				if deleteFlags.leafOnly && hasChildren[col.ID] {
					continue
				}
				if excluded[col.ID] {
					continue
				}
				targets = append(targets, col)
			}
			if deleteFlags.dryRun {
				for _, col := range targets {
					u.PrintInfo(fmt.Sprintf("[dry-run] would delete %d (%q)", col.ID, col.Title))
				}
				u.PrintInfo(fmt.Sprintf("[dry-run] %d empty collection(s) would be pruned", len(targets)))
				return
			}
			var removed int
			for _, col := range targets {
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

		cols, err := collections.ListAll(c)
		if err != nil {
			u.PrintFatal("list collections", err)
		}
		var target *collections.Collection
		for i := range cols {
			if cols[i].ID == deleteFlags.id {
				target = &cols[i]
				break
			}
		}
		if target == nil {
			u.PrintFatal(fmt.Sprintf("collection %d not found", deleteFlags.id), nil)
		}
		if target.Count > 0 && !deleteFlags.force {
			u.PrintFatal(fmt.Sprintf("collection %d has %d item(s); pass --force to delete (items go to Trash)", target.ID, target.Count), nil)
		}

		if deleteFlags.dryRun {
			u.PrintInfo(fmt.Sprintf("[dry-run] would delete %d (%q, count=%d)", target.ID, target.Title, target.Count))
			return
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
	createCmd.Flags().BoolVar(&createFlags.quiet, "quiet", false, "Print only the new collection ID (for shell scripts)")
	_ = createCmd.MarkFlagRequired("title")

	moveCmd.Flags().IntVar(&moveFlags.id, "id", 0, "Collection ID to move")
	moveCmd.Flags().StringVar(&moveFlags.parent, "parent", "", "Target parent ID, or 'root' to promote")
	moveCmd.Flags().StringVar(&moveFlags.fromFile, "from-file", "", "TSV file: <collection_id>\\t<new_parent_id> (0 = root)")
	moveCmd.Flags().BoolVar(&moveFlags.progress, "progress", false, "Show per-row progress on stderr")
	moveCmd.Flags().BoolVar(&moveFlags.dryRun, "dry-run", false, "Preview without writing")

	renameCmd.Flags().IntVar(&renameFlags.id, "id", 0, "Collection ID to rename")
	renameCmd.Flags().StringVar(&renameFlags.to, "to", "", "New title")
	_ = renameCmd.MarkFlagRequired("id")
	_ = renameCmd.MarkFlagRequired("to")

	deleteCmd.Flags().IntVar(&deleteFlags.id, "id", 0, "Collection ID to delete")
	deleteCmd.Flags().BoolVar(&deleteFlags.force, "force", false, "Delete even if non-empty (items → Trash)")
	deleteCmd.Flags().BoolVar(&deleteFlags.empty, "empty", false, "Prune all zero-count collections")
	deleteCmd.Flags().BoolVar(&deleteFlags.leafOnly, "leaf-only", false, "With --empty, only delete collections that also have no child collections")
	deleteCmd.Flags().IntSliceVar(&deleteFlags.excludeIDs, "exclude-ids", nil, "With --empty, comma-separated collection IDs to keep")
	deleteCmd.Flags().BoolVar(&deleteFlags.dryRun, "dry-run", false, "Preview without deleting")
}
