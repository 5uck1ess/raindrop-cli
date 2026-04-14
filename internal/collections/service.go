package collections

import (
	"fmt"

	"github.com/5uck1ess/raindrop-cli/internal/client"
)

type parentRef struct {
	ID int `json:"$id"`
}

type Collection struct {
	ID       int        `json:"_id"`
	Title    string     `json:"title"`
	Count    int        `json:"count"`
	Color    string     `json:"color,omitempty"`
	View     string     `json:"view,omitempty"`
	Parent   *parentRef `json:"parent,omitempty"`
	ParentID int        `json:"-"`
}

type listResponse struct {
	Result bool         `json:"result"`
	Items  []Collection `json:"items"`
}

type itemResponse struct {
	Result bool       `json:"result"`
	Item   Collection `json:"item"`
}

// ListAll returns root + nested collections with ParentID flattened.
func ListAll(c *client.Client) ([]Collection, error) {
	roots, err := fetch(c, "/collections")
	if err != nil {
		return nil, fmt.Errorf("list roots: %w", err)
	}
	children, err := fetch(c, "/collections/childrens")
	if err != nil {
		return nil, fmt.Errorf("list children: %w", err)
	}
	out := make([]Collection, 0, len(roots)+len(children))
	for _, col := range roots {
		col.ParentID = 0
		out = append(out, col)
	}
	for _, col := range children {
		if col.Parent != nil {
			col.ParentID = col.Parent.ID
		}
		out = append(out, col)
	}
	return out, nil
}

func fetch(c *client.Client, path string) ([]Collection, error) {
	var resp listResponse
	if err := c.Do("GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// Create makes a new collection. parentID == 0 creates at root.
func Create(c *client.Client, title string, parentID int, color string) (Collection, error) {
	body := map[string]any{"title": title}
	if parentID != 0 {
		body["parent"] = map[string]any{"$id": parentID}
	}
	if color != "" {
		body["color"] = color
	}
	var resp itemResponse
	if err := c.Do("POST", "/collection", body, &resp); err != nil {
		return Collection{}, err
	}
	return resp.Item, nil
}

// Rename updates the title of collection id.
func Rename(c *client.Client, id int, title string) error {
	body := map[string]any{"title": title}
	path := fmt.Sprintf("/collection/%d", id)
	return c.Do("PUT", path, body, nil)
}

// Reparent moves collection id under parentID. parentID == 0 promotes to root.
func Reparent(c *client.Client, id, parentID int) error {
	var body map[string]any
	if parentID == 0 {
		body = map[string]any{"parent": nil}
	} else {
		body = map[string]any{"parent": map[string]any{"$id": parentID}}
	}
	path := fmt.Sprintf("/collection/%d", id)
	return c.Do("PUT", path, body, nil)
}

// Delete removes a collection. Items are moved to Trash per Raindrop API.
func Delete(c *client.Client, id int) error {
	path := fmt.Sprintf("/collection/%d", id)
	return c.Do("DELETE", path, nil, nil)
}
