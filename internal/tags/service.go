package tags

import (
	"fmt"

	"github.com/5uck1ess/raindrop-cli/internal/client"
)

type Tag struct {
	Name  string `json:"_id"`
	Count int    `json:"count"`
}

type listResponse struct {
	Result bool  `json:"result"`
	Items  []Tag `json:"items"`
}

func List(c *client.Client, collectionID int) ([]Tag, error) {
	path := "/tags"
	if collectionID != 0 {
		path = fmt.Sprintf("/tags/%d", collectionID)
	}
	var resp listResponse
	if err := c.Do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// Merge renames tags[] to target. If tags has one element, it's a rename.
func Merge(c *client.Client, collectionID int, sources []string, target string) error {
	body := map[string]any{
		"tags":    sources,
		"replace": target,
	}
	path := "/tags"
	if collectionID != 0 {
		path = fmt.Sprintf("/tags/%d", collectionID)
	}
	return c.Do("PUT", path, body, nil)
}

func Delete(c *client.Client, collectionID int, names []string) error {
	body := map[string]any{"tags": names}
	path := "/tags"
	if collectionID != 0 {
		path = fmt.Sprintf("/tags/%d", collectionID)
	}
	return c.Do("DELETE", path, body, nil)
}
