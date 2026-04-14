package raindropsCmd

import (
	"fmt"
	"strings"

	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/5uck1ess/raindrop-cli/internal/raindrops"
	u "github.com/5uck1ess/raindrop-cli/utils"
	"github.com/spf13/cobra"
)

var untaggedCmd = &cobra.Command{
	Use:   "untagged",
	Short: "List bookmarks with no tags (client-side filter)",
	Long:  "Raindrop's #untagged search operator is broken via API; this fetches all bookmarks and filters on empty tags client-side.",
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.New()
		if err != nil {
			u.PrintFatal("auth", err)
		}
		all, err := raindrops.ListAll(c, 0, "")
		if err != nil {
			u.PrintFatal("list bookmarks", err)
		}
		var untagged []raindrops.Raindrop
		for _, r := range all {
			if len(r.Tags) == 0 {
				untagged = append(untagged, r)
			}
		}
		if len(untagged) == 0 {
			u.PrintInfo("no untagged bookmarks")
			return
		}

		if u.GlobalForAIFlag {
			fmt.Println("id\tcollection_id\tdomain\ttitle\tlink")
			for _, r := range untagged {
				fmt.Printf("%d\t%d\t%s\t%s\t%s\n", r.ID, r.CollectionID(), r.Domain, sanitize(r.Title), r.Link)
			}
			return
		}

		rows := make([][]string, 0, len(untagged))
		for _, r := range untagged {
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				fmt.Sprintf("%d", r.CollectionID()),
				r.Domain,
				truncate(r.Title, 60),
			})
		}
		u.PrintTable([]string{"ID", "COLLECTION", "DOMAIN", "TITLE"}, rows)
		u.PrintInfo(fmt.Sprintf("%d untagged of %d total", len(untagged), len(all)))
	},
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func init() {
	RaindropsCmd.AddCommand(untaggedCmd)
}
