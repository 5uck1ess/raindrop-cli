package tagsCmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

// Vocab is a local allowlist file of approved tags for automation scripts
// that want to avoid inventing new tag names. It lives in the user config
// dir (no Raindrop API involvement).

var vocabCmd = &cobra.Command{
	Use:   "vocab",
	Short: "Manage a local allowlist of approved tags (for automation)",
	Long: `Vocab is a user-local file at $XDG_CONFIG_HOME/raindrop-cli/vocab.txt
(one tag per line). Tagging scripts can source it to avoid inventing
new tag names. No Raindrop API calls are made.`,
}

var vocabListCmd = &cobra.Command{
	Use:   "list",
	Short: "Print the allowed tags (one per line)",
	Run: func(cmd *cobra.Command, args []string) {
		tags, err := loadVocab()
		if err != nil {
			u.PrintFatal("vocab", err)
		}
		if len(tags) == 0 {
			u.PrintInfo("vocab is empty — use 'tags vocab add <tag,...>'")
			return
		}
		for _, t := range tags {
			fmt.Println(t)
		}
	},
}

var vocabAddFlags struct {
	tags []string
}

var vocabAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add tags to the allowlist (comma-separated)",
	Run: func(cmd *cobra.Command, args []string) {
		if len(vocabAddFlags.tags) == 0 {
			u.PrintFatal("--tag is required", nil)
		}
		current, err := loadVocab()
		if err != nil {
			u.PrintFatal("load vocab", err)
		}
		seen := map[string]bool{}
		for _, t := range current {
			seen[t] = true
		}
		added := 0
		for _, t := range vocabAddFlags.tags {
			t = strings.TrimSpace(t)
			if t == "" || seen[t] {
				continue
			}
			seen[t] = true
			current = append(current, t)
			added++
		}
		if err := saveVocab(current); err != nil {
			u.PrintFatal("save vocab", err)
		}
		u.PrintSuccess(fmt.Sprintf("added %d tag(s); vocab now has %d", added, len(current)))
	},
}

var vocabRemoveFlags struct {
	tags []string
}

var vocabRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove tags from the allowlist",
	Run: func(cmd *cobra.Command, args []string) {
		current, err := loadVocab()
		if err != nil {
			u.PrintFatal("load vocab", err)
		}
		drop := map[string]bool{}
		for _, t := range vocabRemoveFlags.tags {
			drop[strings.TrimSpace(t)] = true
		}
		kept := current[:0]
		removed := 0
		for _, t := range current {
			if drop[t] {
				removed++
				continue
			}
			kept = append(kept, t)
		}
		if err := saveVocab(kept); err != nil {
			u.PrintFatal("save vocab", err)
		}
		u.PrintSuccess(fmt.Sprintf("removed %d tag(s); vocab now has %d", removed, len(kept)))
	},
}

func vocabPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config dir: %w", err)
	}
	return filepath.Join(dir, "raindrop-cli", "vocab.txt"), nil
}

func loadVocab() ([]string, error) {
	path, err := vocabPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t := strings.TrimSpace(sc.Text())
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		out = append(out, t)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return out, nil
}

func saveVocab(tags []string) error {
	path, err := vocabPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	sort.Strings(tags)
	out := strings.Join(tags, "\n") + "\n"
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func init() {
	TagsCmd.AddCommand(vocabCmd)
	vocabCmd.AddCommand(vocabListCmd, vocabAddCmd, vocabRemoveCmd)
	vocabAddCmd.Flags().StringSliceVar(&vocabAddFlags.tags, "tag", nil, "Tags to add (comma-separated)")
	_ = vocabAddCmd.MarkFlagRequired("tag")
	vocabRemoveCmd.Flags().StringSliceVar(&vocabRemoveFlags.tags, "tag", nil, "Tags to remove (comma-separated)")
	_ = vocabRemoveCmd.MarkFlagRequired("tag")
}
