package raindropsCmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/raindrops"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

var verifyFlags struct {
	fromFile string
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check each bookmark in a TSV plan has the expected tags applied",
	Long: `Reads the same TSV format as 'bookmarks tag --from-file'
(<id>\t<tags> or <id>\t<collection_id>\t<tags>) and fetches each bookmark
to confirm every expected tag is actually present. Reports a summary on
stdout and a TSV of mismatches on stderr.`,
	Run: func(cmd *cobra.Command, args []string) {
		entries, err := parseTSV(verifyFlags.fromFile)
		if err != nil {
			u.PrintFatal("parse tsv", err)
		}
		if len(entries) == 0 {
			u.PrintInfo("no entries")
			return
		}
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}

		pass := 0
		fail := 0
		missingRows := make([][]string, 0)
		for _, e := range entries {
			r, err := raindrops.Get(c, e.id)
			if err != nil {
				u.PrintWarn(fmt.Sprintf("id=%d fetch", e.id), err)
				fail++
				continue
			}
			have := map[string]bool{}
			for _, t := range r.Tags {
				have[t] = true
			}
			var missing []string
			for _, want := range e.tags {
				if !have[want] {
					missing = append(missing, want)
				}
			}
			if len(missing) == 0 {
				pass++
				continue
			}
			fail++
			sort.Strings(missing)
			missingRows = append(missingRows, []string{
				fmt.Sprintf("%d", e.id),
				strings.Join(r.Tags, ","),
				strings.Join(missing, ","),
			})
		}

		if len(missingRows) > 0 {
			fmt.Fprintln(os.Stderr, "id\tactual_tags\tmissing_tags")
			for _, row := range missingRows {
				fmt.Fprintln(os.Stderr, strings.Join(row, "\t"))
			}
		}
		u.PrintSuccess(fmt.Sprintf("verify: %d/%d passed (%d failed)", pass, pass+fail, fail))
	},
}

func init() {
	RaindropsCmd.AddCommand(verifyCmd)
	verifyCmd.Flags().StringVar(&verifyFlags.fromFile, "from-file", "", "TSV plan to verify (same format as 'tag --from-file')")
	_ = verifyCmd.MarkFlagRequired("from-file")
}
