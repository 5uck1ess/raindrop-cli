package raindropsCmd

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/raindrops"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

var moveFlags struct {
	id             int
	to             int
	fromFile       string
	fromCollection int
	filter         string
	progress       bool
	dryRun         bool
}

var moveCmd = &cobra.Command{
	Use:   "move",
	Short: "Reparent bookmarks to another collection (single id, TSV batch, or whole-collection)",
	Long: `Three modes:

  --id N --to M                              move one bookmark
  --from-file plan.tsv                       TSV: <id>\t<new_collection_id>
  --from-collection X --to Y [--filter re]   move every (matching) bookmark in X to Y

Batch modes group by target collection and use bulk PUT /raindrops/{cid}
with {collection:{$id:M}} — 100 ids per call.`,
	Run: func(cmd *cobra.Command, args []string) {
		switch {
		case moveFlags.fromFile != "":
			runMoveFromFile()
		case moveFlags.fromCollection != 0:
			if moveFlags.to == 0 {
				u.PrintFatal("--from-collection requires --to", nil)
			}
			runMoveFromCollection()
		case moveFlags.id != 0:
			if moveFlags.to == 0 {
				u.PrintFatal("--id requires --to", nil)
			}
			runMoveSingle()
		default:
			u.PrintFatal("need --id, --from-file, or --from-collection", nil)
		}
	},
}

func runMoveSingle() {
	if moveFlags.dryRun {
		u.PrintInfo(fmt.Sprintf("[dry-run] move id=%d → cid=%d", moveFlags.id, moveFlags.to))
		return
	}
	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}
	if err := raindrops.Move(c, moveFlags.id, moveFlags.to); err != nil {
		u.PrintFatal("move", err)
	}
	u.PrintSuccess(fmt.Sprintf("moved id=%d → cid=%d", moveFlags.id, moveFlags.to))
}

func runMoveFromFile() {
	entries, err := parseMoveTSV(moveFlags.fromFile)
	if err != nil {
		u.PrintFatal("parse tsv", err)
	}
	if len(entries) == 0 {
		u.PrintInfo("no entries")
		return
	}

	// Group ids by target cid (same destination → same bulk call).
	byTarget := map[int][]int{}
	for _, e := range entries {
		byTarget[e.newCID] = append(byTarget[e.newCID], e.id)
	}

	if moveFlags.dryRun {
		for cid, ids := range byTarget {
			u.PrintInfo(fmt.Sprintf("[dry-run] → cid=%d ids=%d", cid, len(ids)))
		}
		u.PrintInfo(fmt.Sprintf("[dry-run] %d bookmark(s) across %d target cid(s)", len(entries), len(byTarget)))
		return
	}

	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}

	ok, fail := 0, 0
	idx := 0
	for cid, ids := range byTarget {
		idx++
		for start := 0; start < len(ids); start += bulkChunkSize {
			end := start + bulkChunkSize
			if end > len(ids) {
				end = len(ids)
			}
			chunk := ids[start:end]
			// We don't know the source cid for each id; Raindrop's bulk PUT
			// accepts cid=0 as a fallback scope ONLY when explicit ids are
			// given. If this rejects with 400, fall back per-item below.
			if err := raindrops.MoveMany(c, 0, chunk, cid); err != nil {
				for _, id := range chunk {
					if err := raindrops.Move(c, id, cid); err != nil {
						u.PrintWarn(fmt.Sprintf("id=%d → cid=%d", id, cid), err)
						fail++
					} else {
						ok++
					}
				}
			} else {
				ok += len(chunk)
			}
			if moveFlags.progress {
				fmt.Fprintf(os.Stderr, "[target %d/%d] cid=%d items=%d\n", idx, len(byTarget), cid, len(chunk))
			}
		}
	}
	u.PrintSuccess(fmt.Sprintf("move: %d applied, %d failed across %d target(s)", ok, fail, len(byTarget)))
}

func runMoveFromCollection() {
	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}
	all, err := raindrops.ListAll(c, moveFlags.fromCollection, "")
	if err != nil {
		u.PrintFatal("list source", err)
	}

	var re *regexp.Regexp
	if moveFlags.filter != "" {
		re, err = regexp.Compile(moveFlags.filter)
		if err != nil {
			u.PrintFatal("bad --filter regex", err)
		}
	}

	var ids []int
	for _, r := range all {
		if re != nil && !re.MatchString(r.Title) && !re.MatchString(r.Link) && !re.MatchString(r.Domain) {
			continue
		}
		ids = append(ids, r.ID)
	}
	if len(ids) == 0 {
		u.PrintInfo("no matching bookmarks")
		return
	}

	if moveFlags.dryRun {
		u.PrintInfo(fmt.Sprintf("[dry-run] move %d bookmark(s) %d → %d", len(ids), moveFlags.fromCollection, moveFlags.to))
		return
	}

	ok, fail := 0, 0
	for start := 0; start < len(ids); start += bulkChunkSize {
		end := start + bulkChunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		if err := raindrops.MoveMany(c, moveFlags.fromCollection, chunk, moveFlags.to); err != nil {
			u.PrintWarn(fmt.Sprintf("chunk %d-%d", start, end), err)
			fail += len(chunk)
		} else {
			ok += len(chunk)
		}
		if moveFlags.progress {
			fmt.Fprintf(os.Stderr, "[chunk %d/%d] items=%d\n", (start/bulkChunkSize)+1, (len(ids)+bulkChunkSize-1)/bulkChunkSize, len(chunk))
		}
	}
	u.PrintSuccess(fmt.Sprintf("moved: %d ok, %d failed (%d → %d)", ok, fail, moveFlags.fromCollection, moveFlags.to))
}

type moveEntry struct {
	id, newCID int
}

func parseMoveTSV(path string) ([]moveEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var out []moveEntry
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
			return nil, fmt.Errorf("line %d: expected <id>\\t<new_collection_id>", line)
		}
		id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: bad id: %w", line, err)
		}
		cid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("line %d: bad new_collection_id: %w", line, err)
		}
		out = append(out, moveEntry{id: id, newCID: cid})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

func init() {
	RaindropsCmd.AddCommand(moveCmd)
	moveCmd.Flags().IntVar(&moveFlags.id, "id", 0, "Bookmark ID to move (single-item)")
	moveCmd.Flags().IntVar(&moveFlags.to, "to", 0, "Target collection ID")
	moveCmd.Flags().StringVar(&moveFlags.fromFile, "from-file", "", "TSV file: <id>\\t<new_collection_id>")
	moveCmd.Flags().IntVar(&moveFlags.fromCollection, "from-collection", 0, "Source collection ID (move every item in it)")
	moveCmd.Flags().StringVar(&moveFlags.filter, "filter", "", "With --from-collection, only move items whose title/link/domain matches this regex")
	moveCmd.Flags().BoolVar(&moveFlags.progress, "progress", false, "Show chunk progress on stderr")
	moveCmd.Flags().BoolVar(&moveFlags.dryRun, "dry-run", false, "Preview without writing")
}
