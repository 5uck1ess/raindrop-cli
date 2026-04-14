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

type itemResponse struct {
	Result bool     `json:"result"`
	Item   Raindrop `json:"item"`
}

// Get fetches a single raindrop by id.
func Get(c *client.Client, id int) (Raindrop, error) {
	var resp itemResponse
	path := fmt.Sprintf("/raindrop/%d", id)
	if err := c.Do("GET", path, nil, &resp); err != nil {
		return Raindrop{}, err
	}
	return resp.Item, nil
}

// SetTags replaces the tags on a single raindrop.
func SetTags(c *client.Client, id int, tags []string) error {
	if tags == nil {
		tags = []string{}
	}
	body := map[string]any{"tags": tags}
	path := fmt.Sprintf("/raindrop/%d", id)
	return c.Do("PUT", path, body, nil)
}

// ListAll pages through /raindrops/0 and returns every raindrop.
func ListAll(c *client.Client, search string) ([]Raindrop, error) {
	const perPage = 50
	var all []Raindrop
	for page := 0; ; page++ {
		items, total, err := List(c, 0, search, page, perPage)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		all = append(all, items...)
		if len(all) >= total || len(items) == 0 {
			break
		}
	}
	return all, nil
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
