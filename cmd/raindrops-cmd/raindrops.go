package raindropsCmd

import "github.com/spf13/cobra"

var RaindropsCmd = &cobra.Command{
	Use:     "bookmarks",
	Aliases: []string{"raindrops", "bm"},
	Short:   "List, search, update, and delete bookmarks",
}
