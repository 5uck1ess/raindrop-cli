package raindropsCmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
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

const bulkChunkSize = 100

var tagFlags struct {
	id                 int
	add                []string
	remove             []string
	set                []string
	fromFile           string
	fromCollectionMap  string
	untaggedOnly       bool
	modeStr            string
	noBulk             bool
	progress           bool
	dryRun             bool
}

var tagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Add, remove, or replace tags on bookmarks",
	Long: `Update tags on one bookmark (--id) or many (--from-file TSV).

TSV formats accepted (# comments ignored):
  <id>\t<tag1,tag2,...>
  <id>\t<collection_id>\t<tag1,tag2,...>

Single-id mode: pass one of --add / --remove / --set with comma-separated tags.
Batch mode (--from-file): pass --mode add|set|remove. add/set use the bulk
PUT /raindrops/{cid} endpoint (up to 100 ids per call, grouped by collection
and tag-set) — pass --no-bulk to force per-item writes. --mode remove always
takes the per-item path because the bulk endpoint cannot remove specific tags.`,
	Run: func(cmd *cobra.Command, args []string) {
		if tagFlags.fromCollectionMap != "" {
			runFromCollectionMap()
			return
		}
		if tagFlags.fromFile != "" {
			m, err := batchMode()
			if err != nil {
				u.PrintFatal("mode", err)
			}
			runBatch(m)
			return
		}

		m, tags, err := singleMode()
		if err != nil {
			u.PrintFatal("mode", err)
		}
		if tagFlags.id == 0 {
			u.PrintFatal("need --id or --from-file", nil)
		}
		runSingle(tagFlags.id, m, tags)
	},
}

func singleMode() (mode, []string, error) {
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

func batchMode() (mode, error) {
	switch strings.ToLower(tagFlags.modeStr) {
	case "add":
		return modeAdd, nil
	case "set":
		return modeSet, nil
	case "remove":
		return modeRemove, nil
	case "":
		return 0, fmt.Errorf("--from-file requires --mode add|set|remove")
	default:
		return 0, fmt.Errorf("unknown --mode %q (use add|set|remove)", tagFlags.modeStr)
	}
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
	entries, err := parseTSV(tagFlags.fromFile)
	if err != nil {
		u.PrintFatal("parse tsv", err)
	}
	if len(entries) == 0 {
		u.PrintInfo("no entries")
		return
	}

	// Remove must stay per-item — bulk cannot remove specific tags.
	if m == modeRemove || tagFlags.noBulk {
		runBatchPerItem(entries, m)
		return
	}

	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}

	// Preflight: resolve any entries missing collection_id by paginating
	// /raindrops/0 once and building an id→cid map.
	missing := 0
	for _, e := range entries {
		if e.collectionID == 0 {
			missing++
		}
	}
	if missing > 0 {
		u.PrintInfo(fmt.Sprintf("preflight: resolving collection_id for %d entries…", missing))
		all, err := raindrops.ListAll(c, 0, "")
		if err != nil {
			u.PrintFatal("preflight list", err)
		}
		cidByID := make(map[int]int, len(all))
		for _, r := range all {
			cidByID[r.ID] = r.CollectionID()
		}
		for i := range entries {
			if entries[i].collectionID == 0 {
				entries[i].collectionID = cidByID[entries[i].id]
			}
		}
	}

	// Bucket by (cid, tag-set). Raindrop's bulk PUT applies the same tags
	// to every id in the call, so each bucket = one tag-set for one cid.
	type bucketKey struct {
		cid  int
		tags string
	}
	type bucket struct {
		cid  int
		tags []string
		ids  []int
	}
	buckets := map[bucketKey]*bucket{}
	var skipped int
	for _, e := range entries {
		if e.collectionID == 0 {
			skipped++
			continue
		}
		key := bucketKey{cid: e.collectionID, tags: joinSorted(e.tags)}
		b, ok := buckets[key]
		if !ok {
			b = &bucket{cid: e.collectionID, tags: dedupTags(e.tags)}
			buckets[key] = b
		}
		b.ids = append(b.ids, e.id)
	}
	if skipped > 0 {
		u.PrintWarn(fmt.Sprintf("skipping %d entry(ies) with unresolved collection_id", skipped), nil)
	}

	total := 0
	for _, b := range buckets {
		total += len(b.ids)
	}

	if tagFlags.dryRun {
		for _, b := range buckets {
			for chunkStart := 0; chunkStart < len(b.ids); chunkStart += bulkChunkSize {
				end := chunkStart + bulkChunkSize
				if end > len(b.ids) {
					end = len(b.ids)
				}
				u.PrintInfo(fmt.Sprintf("[dry-run] %s cid=%d tags=%v ids=%d", modeLabel(m), b.cid, b.tags, end-chunkStart))
			}
		}
		u.PrintInfo(fmt.Sprintf("[dry-run] %d entries across %d bucket(s)", total, len(buckets)))
		return
	}

	// Deterministic bucket order so --progress ticks match user expectations.
	bucketKeys := make([]bucketKey, 0, len(buckets))
	for k := range buckets {
		bucketKeys = append(bucketKeys, k)
	}
	sort.Slice(bucketKeys, func(i, j int) bool {
		if bucketKeys[i].cid != bucketKeys[j].cid {
			return bucketKeys[i].cid < bucketKeys[j].cid
		}
		return bucketKeys[i].tags < bucketKeys[j].tags
	})

	totalBuckets := len(buckets)
	bucketIdx := 0
	ok := 0
	fail := 0
	for _, key := range bucketKeys {
		b := buckets[key]
		bucketIdx++
		for chunkStart := 0; chunkStart < len(b.ids); chunkStart += bulkChunkSize {
			end := chunkStart + bulkChunkSize
			if end > len(b.ids) {
				end = len(b.ids)
			}
			chunk := b.ids[chunkStart:end]
			if err := applyBulk(c, b.cid, chunk, b.tags, m); err != nil {
				u.PrintWarn(fmt.Sprintf("cid=%d chunk %d-%d", b.cid, chunkStart, end), err)
				fail += len(chunk)
			} else {
				ok += len(chunk)
			}
			if tagFlags.progress {
				fmt.Fprintf(os.Stderr, "[bucket %d/%d] cid=%d items=%d tags=%v\n",
					bucketIdx, totalBuckets, b.cid, len(chunk), b.tags)
			}
		}
	}
	u.PrintSuccess(fmt.Sprintf("bulk %s: %d applied, %d failed across %d bucket(s)", modeLabel(m), ok, fail, totalBuckets))
}

// applyBulk issues 1 or 2 PUT /raindrops/{cid} calls covering the chunk.
// add  → append tags
// set  → clear ([]), then append new tags
func applyBulk(c *client.Client, cid int, ids []int, tags []string, m mode) error {
	switch m {
	case modeAdd:
		return raindrops.UpdateMany(c, cid, ids, map[string]any{"tags": tags})
	case modeSet:
		if err := raindrops.UpdateMany(c, cid, ids, map[string]any{"tags": []string{}}); err != nil {
			return fmt.Errorf("clear: %w", err)
		}
		if len(tags) == 0 {
			return nil
		}
		return raindrops.UpdateMany(c, cid, ids, map[string]any{"tags": tags})
	}
	return fmt.Errorf("bulk does not support mode %s", modeLabel(m))
}

// runFromCollectionMap applies tags based on collection membership. TSV
// rows are <collection_id>\t<tag1,tag2,...>. For each row, every bookmark
// in that collection (optionally filtered to untagged) gets the tags
// appended in a single bulk PUT per 100-id chunk.
func runFromCollectionMap() {
	entries, err := parseCollectionMapTSV(tagFlags.fromCollectionMap)
	if err != nil {
		u.PrintFatal("parse collection-map", err)
	}
	if len(entries) == 0 {
		u.PrintInfo("no entries")
		return
	}

	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}

	type plan struct {
		cid  int
		tags []string
		ids  []int
	}
	var plans []plan
	totalIDs := 0
	for _, e := range entries {
		all, err := raindrops.ListAll(c, e.cid, "")
		if err != nil {
			u.PrintWarn(fmt.Sprintf("cid=%d list", e.cid), err)
			continue
		}
		var ids []int
		for _, r := range all {
			if tagFlags.untaggedOnly && len(r.Tags) > 0 {
				continue
			}
			ids = append(ids, r.ID)
		}
		if len(ids) == 0 {
			u.PrintInfo(fmt.Sprintf("cid=%d: no matching bookmarks", e.cid))
			continue
		}
		plans = append(plans, plan{cid: e.cid, tags: e.tags, ids: ids})
		totalIDs += len(ids)
	}

	if tagFlags.dryRun {
		for _, p := range plans {
			u.PrintInfo(fmt.Sprintf("[dry-run] cid=%d → tags=%v on %d bookmark(s)", p.cid, p.tags, len(p.ids)))
		}
		u.PrintInfo(fmt.Sprintf("[dry-run] %d collection(s), %d bookmark(s) total", len(plans), totalIDs))
		return
	}

	ok := 0
	fail := 0
	for i, p := range plans {
		for chunkStart := 0; chunkStart < len(p.ids); chunkStart += bulkChunkSize {
			end := chunkStart + bulkChunkSize
			if end > len(p.ids) {
				end = len(p.ids)
			}
			chunk := p.ids[chunkStart:end]
			if err := raindrops.UpdateMany(c, p.cid, chunk, map[string]any{"tags": p.tags}); err != nil {
				u.PrintWarn(fmt.Sprintf("cid=%d chunk %d-%d", p.cid, chunkStart, end), err)
				fail += len(chunk)
			} else {
				ok += len(chunk)
			}
			if tagFlags.progress {
				fmt.Fprintf(os.Stderr, "[collection %d/%d] cid=%d items=%d tags=%v\n",
					i+1, len(plans), p.cid, len(chunk), p.tags)
			}
		}
	}
	u.PrintSuccess(fmt.Sprintf("collection-map: %d applied, %d failed across %d collection(s)", ok, fail, len(plans)))
}

type cmapEntry struct {
	cid  int
	tags []string
}

func parseCollectionMapTSV(path string) ([]cmapEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	var out []cmapEntry
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
			return nil, fmt.Errorf("line %d: expected <collection_id>\\t<tags>", line)
		}
		cid, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("line %d: bad collection_id: %w", line, err)
		}
		out = append(out, cmapEntry{cid: cid, tags: splitTags(parts[1])})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

func runBatchPerItem(entries []tsvEntry, m mode) {
	if tagFlags.dryRun {
		for _, e := range entries {
			u.PrintInfo(fmt.Sprintf("[dry-run] id=%d %s %v", e.id, modeLabel(m), e.tags))
		}
		u.PrintInfo(fmt.Sprintf("[dry-run] %d entries (per-item)", len(entries)))
		return
	}
	c, err := client.New()
	if err != nil {
		u.PrintFatal("auth", err)
	}
	var ok, fail int
	total := len(entries)
	for i, e := range entries {
		if err := applyOne(c, e.id, m, e.tags); err != nil {
			u.PrintWarn(fmt.Sprintf("id=%d", e.id), err)
			fail++
		} else {
			ok++
		}
		if tagFlags.progress {
			fmt.Fprintf(os.Stderr, "\r[%d/%d] tagged          ", i+1, total)
		}
	}
	if tagFlags.progress {
		fmt.Fprintln(os.Stderr)
	}
	u.PrintSuccess(fmt.Sprintf("per-item %s: %d applied, %d failed", modeLabel(m), ok, fail))
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

func dedupTags(tags []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

func joinSorted(tags []string) string {
	cp := append([]string(nil), tags...)
	sort.Strings(cp)
	return strings.Join(cp, ",")
}

type tsvEntry struct {
	id           int
	collectionID int // 0 if absent in the TSV — preflight will fill in
	tags         []string
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
		raw := strings.TrimRight(sc.Text(), "\r\n")
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		parts := strings.Split(raw, "\t")
		switch len(parts) {
		case 2:
			id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("line %d: bad id: %w", line, err)
			}
			out = append(out, tsvEntry{id: id, tags: splitTags(parts[1])})
		case 3:
			id, err := strconv.Atoi(strings.TrimSpace(parts[0]))
			if err != nil {
				return nil, fmt.Errorf("line %d: bad id: %w", line, err)
			}
			cid, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("line %d: bad collection_id: %w", line, err)
			}
			out = append(out, tsvEntry{id: id, collectionID: cid, tags: splitTags(parts[2])})
		default:
			return nil, fmt.Errorf("line %d: expected 2 or 3 tab-separated columns", line)
		}
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
	tagCmd.Flags().StringSliceVar(&tagFlags.add, "add", nil, "Tags to append (single-id mode)")
	tagCmd.Flags().StringSliceVar(&tagFlags.remove, "remove", nil, "Tags to remove (single-id mode)")
	tagCmd.Flags().StringSliceVar(&tagFlags.set, "set", nil, "Tags to replace (single-id mode)")
	tagCmd.Flags().StringVar(&tagFlags.fromFile, "from-file", "", "TSV file: <id>\\t<tags> or <id>\\t<collection_id>\\t<tags>")
	tagCmd.Flags().StringVar(&tagFlags.fromCollectionMap, "from-collection-map", "", "TSV file: <collection_id>\\t<tags> — append tags to every bookmark in each collection")
	tagCmd.Flags().BoolVar(&tagFlags.untaggedOnly, "untagged-only", false, "With --from-collection-map, only tag bookmarks whose tags[] is empty")
	tagCmd.Flags().StringVar(&tagFlags.modeStr, "mode", "", "Batch mode: add | set | remove (with --from-file)")
	tagCmd.Flags().BoolVar(&tagFlags.noBulk, "no-bulk", false, "Force per-item writes in batch mode (disables the bulk PUT path)")
	tagCmd.Flags().BoolVar(&tagFlags.progress, "progress", false, "Show [N/total] progress on stderr during batch")
	tagCmd.Flags().BoolVar(&tagFlags.dryRun, "dry-run", false, "Preview without writing")
}
