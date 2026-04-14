package raindrops

import (
	"fmt"
	"net/url"

	"github.com/5uck1ess/raindrop-cli/internal/client"
)

type Raindrop struct {
	ID      int      `json:"_id"`
	Title   string   `json:"title"`
	Link    string   `json:"link"`
	Domain  string   `json:"domain"`
	Tags    []string `json:"tags"`
	Created string   `json:"created"`
}

type listResponse struct {
	Result bool       `json:"result"`
	Items  []Raindrop `json:"items"`
	Count  int        `json:"count"`
}

// List fetches raindrops in a collection. collectionID 0 = all, -1 = unsorted,
// -99 = trash. search is an optional Raindrop search query.
func List(c *client.Client, collectionID int, search string, page, perPage int) ([]Raindrop, int, error) {
	q := url.Values{}
	if search != "" {
		q.Set("search", search)
	}
	q.Set("page", fmt.Sprintf("%d", page))
	q.Set("perpage", fmt.Sprintf("%d", perPage))

	var resp listResponse
	path := fmt.Sprintf("/raindrops/%d?%s", collectionID, q.Encode())
	if err := c.Do("GET", path, nil, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Items, resp.Count, nil
}

// UpdateMany updates raindrops matching ids (max 100). fields is the JSON body.
func UpdateMany(c *client.Client, collectionID int, ids []int, fields map[string]any) error {
	body := map[string]any{"ids": ids}
	for k, v := range fields {
		body[k] = v
	}
	path := fmt.Sprintf("/raindrops/%d", collectionID)
	return c.Do("PUT", path, body, nil)
}

// DeleteMany deletes raindrops by ids (max 100).
func DeleteMany(c *client.Client, collectionID int, ids []int) error {
	body := map[string]any{"ids": ids}
	path := fmt.Sprintf("/raindrops/%d", collectionID)
	return c.Do("DELETE", path, body, nil)
}
