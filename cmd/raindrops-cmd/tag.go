package raindropsCmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/raindrops"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

type mode int

const (
	modeAdd mode = iota
	modeRemove
	modeSet
)

var tagFlags struct {
	id       int
	add      []string
	remove   []string
	set      []string
	fromFile string
	dryRun   bool
}

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Add, remove, or replace tags on bookmarks",
	Long: `Update tags on one bookmark (--id) or many (--from-file TSV).

TSV format: <id>\t<tag1,tag2,tag3> (one per line, # comments ignored).
With --from-file, pair exactly one of --add or --set (not --remove).`,
	Run: func(cmd *cobra.Command, args []string) {
		m, tags, err := chosenMode()
		if err != nil {
			u.PrintFatal("mode", err)
		}

		if tagFlags.fromFile != "" {
			runBatch(m)
			return
		}
		if tagFlags.id == 0 {
			u.PrintFatal("need --id or --from-file", nil)
		}
		runSingle(tagFlags.id, m, tags)
	},
}

func chosenMode() (mode, []string, error) {
	set := 0
	var m mode
	var tags []string
	if len(tagFlags.add) > 0 {
		set++
		m, tags = modeAdd, tagFlags.add
	}
	if len(tagFlags.remove) > 0 {
		set++
		m, tags = modeRemove, tagFlags.remove
	}
	if len(tagFlags.set) > 0 {
		set++
		m, tags = modeSet, tagFlags.set
	}
	if set == 0 {
		return 0, nil, fmt.Errorf("need one of --add, --remove, --set")
	}
	if set > 1 {
		return 0, nil, fmt.Errorf("--add, --remove, --set are mutually exclusive")
	}
	return m, tags, nil
}

func runSingle(id int, m mode, tags []string) {
	if tagFlags.dryRun {
		u.PrintInfo(fmt.Sprintf("[dry-run] id=%d %s %v", id, modeLabel(m), tags))
		return
	}
	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}
	if err := applyOne(c, id, m, tags); err != nil {
		u.PrintFatal(fmt.Sprintf("tag id=%d", id), err)
	}
	u.PrintSuccess(fmt.Sprintf("id=%d %s %v", id, modeLabel(m), tags))
}

func runBatch(m mode) {
	if m == modeRemove {
		u.PrintFatal("--from-file does not support --remove (use --id per item)", nil)
	}
	entries, err := parseTSV(tagFlags.fromFile)
	if err != nil {
		u.PrintFatal("parse tsv", err)
	}
	if len(entries) == 0 {
		u.PrintInfo("no entries")
		return
	}
	if tagFlags.dryRun {
		for _, e := range entries {
			u.PrintInfo(fmt.Sprintf("[dry-run] id=%d %s %v", e.id, modeLabel(m), e.tags))
		}
		u.PrintInfo(fmt.Sprintf("[dry-run] %d entries", len(entries)))
		return
	}
	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}
	var ok, fail int
	for _, e := range entries {
		if err := applyOne(c, e.id, m, e.tags); err != nil {
			u.PrintWarn(fmt.Sprintf("id=%d", e.id), err)
			fail++
			continue
		}
		ok++
	}
	u.PrintSuccess(fmt.Sprintf("applied %d, failed %d", ok, fail))
}

// applyOne routes a single-id update. For --add and --remove we read-modify-write
// via PUT /raindrop/{id} because the bulk endpoint only appends (not removes)
// and doesn't support arbitrary per-id tag sets.
func applyOne(c *client.Client, id int, m mode, tags []string) error {
	switch m {
	case modeSet:
		return raindrops.SetTags(c, id, tags)
	case modeAdd:
		cur, err := raindrops.Get(c, id)
		if err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		return raindrops.SetTags(c, id, mergeTags(cur.Tags, tags))
	case modeRemove:
		cur, err := raindrops.Get(c, id)
		if err != nil {
			return fmt.Errorf("fetch: %w", err)
		}
		return raindrops.SetTags(c, id, diffTags(cur.Tags, tags))
	}
	return fmt.Errorf("unknown mode")
}

func mergeTags(existing, add []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(existing)+len(add))
	for _, t := range existing {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, t := range add {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func diffTags(existing, remove []string) []string {
	drop := map[string]bool{}
	for _, t := range remove {
		drop[t] = true
	}
	out := make([]string, 0, len(existing))
	for _, t := range existing {
		if !drop[t] {
			out = append(out, t)
		}
	}
	return out
}

type tsvEntry struct {
	id   int
	tags []string
}

func parseTSV(path string) ([]tsvEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var out []tsvEntry
	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		parts := strings.SplitN(raw, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("line %d: expected <id>\\t<tags>", line)
		}
		id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: bad id: %w", line, err)
		}
		tags := splitTags(parts[1])
		out = append(out, tsvEntry{id: id, tags: tags})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

func splitTags(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func modeLabel(m mode) string {
	switch m {
	case modeAdd:
		return "add"
	case modeRemove:
		return "remove"
	case modeSet:
		return "set"
	}
	return "?"
}

func init() {
	RaindropsCmd.AddCommand(tagCmd)
	tagCmd.Flags().IntVar(&tagFlags.id, "id", 0, "Bookmark ID (single-item mode)")
	tagCmd.Flags().StringSliceVar(&tagFlags.add, "add", nil, "Tags to append (comma-separated)")
	tagCmd.Flags().StringSliceVar(&tagFlags.remove, "remove", nil, "Tags to remove (comma-separated)")
	tagCmd.Flags().StringSliceVar(&tagFlags.set, "set", nil, "Tags to replace (comma-separated)")
	tagCmd.Flags().StringVar(&tagFlags.fromFile, "from-file", "", "TSV file: <id>\\t<tag1,tag2,...>")
	tagCmd.Flags().BoolVar(&tagFlags.dryRun, "dry-run", false, "Preview without writing")
}
