package soulseek

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	host string
	http http.Client
}

func New(host string) *Client {
	return &Client{host: host}
}

func (c *Client) Ping() error {
	req, _ := http.NewRequest("GET", c.host+"/api/v0/application", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

type SearchResult struct {
	ID            string     `json:"id"`
	SearchText    string     `json:"searchText"`
	ResponseCount int        `json:"responseCount"`
	Responses     []Response `json:"responses"`
	State         string     `json:"state"`
	StartedAt     time.Time  `json:"startedAt"`
	EndedAt       time.Time  `json:"endedAt"`
}

type Response struct {
	FileCount         int    `json:"fileCount"`
	QueueLength       int    `json:"queueLength"`
	HasFreeUploadSlot bool   `json:"hasFreeUploadSlot"`
	UploadSpeed       int    `json:"uploadSpeed"`
	Username          string `json:"username"`
	Files             []File `json:"files"`
}

type File struct {
	Filename string `json:"filename"`
	Size     int    `json:"size"`
	IsLocked bool   `json:"isLocked"`
}

func (c *Client) Search(text string) (*SearchResult, error) {
	reqBody := map[string]string{"searchText": text}
	buf, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", c.host+"/api/v0/searches", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var res SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) SearchResult(id string) (*SearchResult, error) {
	req, _ := http.NewRequest("GET", c.host+"/api/v0/searches/"+id+"?includeResponses=true", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var res SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Client) Transfer(username, filename string, size int) error {
	endpoint := fmt.Sprintf("%s/api/v0/transfers/downloads/%s", c.host, url.PathEscape(username))
	payload, _ := json.Marshal([]map[string]any{{"filename": filename, "size": size}})
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

// ListSearches retrieves all searches from the Soulseek daemon
func (c *Client) ListSearches() ([]SearchResult, error) {
	req, _ := http.NewRequest("GET", c.host+"/api/v0/searches", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var searches []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&searches); err != nil {
		return nil, err
	}
	return searches, nil
}

// DeleteSearch removes a specific search by ID
func (c *Client) DeleteSearch(id string) error {
	req, _ := http.NewRequest("DELETE", c.host+"/api/v0/searches/"+id, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// DeleteAllSearches removes all searches from the Soulseek daemon
func (c *Client) DeleteAllSearches() error {
	searches, err := c.ListSearches()
	if err != nil {
		return fmt.Errorf("failed to list searches: %w", err)
	}
	
	for _, search := range searches {
		if err := c.DeleteSearch(search.ID); err != nil {
			return fmt.Errorf("failed to delete search %s: %w", search.ID, err)
		}
	}
	
	return nil
}
