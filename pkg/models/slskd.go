package models

type SearchResult struct {
	Username string `json:"username"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Speed    int    `json:"speed"`
	Quality  int    `json:"quality"`
}

type SearchStatus struct {
	ID        string         `json:"id"`
	Query     string         `json:"query"`
	State     string         `json:"state"`
	Results   []SearchResult `json:"results"`
	Completed bool           `json:"completed"`
}

type DownloadRequest struct {
	Username string `json:"username"`
	Filename string `json:"filename"`
}
