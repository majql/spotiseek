package ApiClients

import (
	"bytes"
	"encoding/json"
	json2 "encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

type SoulseekService struct {
	httpHost   string
	httpClient http.Client
}

type Soulseek interface {
	Search(query string) SearchResult
	GetSearchResult(searchId string) SearchResult
	Transfer(username string, downloadId string, fileSize int) string
	Ping()
}

type SearchResult struct {
	EndedAt         time.Time   `json:"endedAt"`
	FileCount       int         `json:"fileCount"`
	ID              string      `json:"id"`
	IsComplete      bool        `json:"isComplete"`
	LockedFileCount int         `json:"lockedFileCount"`
	ResponseCount   int         `json:"responseCount"`
	Responses       []Responses `json:"responses"`
	SearchText      string      `json:"searchText"`
	StartedAt       time.Time   `json:"startedAt"`
	State           string      `json:"state"`
	Token           int         `json:"token"`
}

type Responses struct {
	FileCount         int    `json:"fileCount"`
	Files             []File `json:"files"`
	HasFreeUploadSlot bool   `json:"hasFreeUploadSlot"`
	LockedFileCount   int    `json:"lockedFileCount"`
	LockedFiles       []any  `json:"lockedFiles"`
	QueueLength       int    `json:"queueLength"`
	Token             int    `json:"token"`
	UploadSpeed       int    `json:"uploadSpeed"`
	Username          string `json:"username"`
}

type File struct {
	BitRate   int    `json:"bitRate"`
	Code      int    `json:"code"`
	Extension string `json:"extension"`
	Filename  string `json:"filename"`
	Length    int    `json:"length"`
	Size      int    `json:"size"`
	IsLocked  bool   `json:"isLocked"`
}

func NewSoulseek(host string) *SoulseekService {
	ss := &SoulseekService{
		httpHost:   host,
		httpClient: http.Client{},
	}

	return ss
}

func (ss *SoulseekService) Ping() {
	apiEndpoint := "/api/v0/application"
	request, _ := http.NewRequest("GET", ss.httpHost+apiEndpoint, nil)
	_, err := ss.httpClient.Do(request)
	if err != nil {
		fmt.Println("Can't connect to Slskd. Quitting.")
		os.Exit(3)
	}
}

func (ss *SoulseekService) Search(query string) SearchResult {
	apiEndpoint := "/api/v0/searches"

	var jsonData = []byte(`{
		"searchText": "` + query + `"
	}`)
	request, err := http.NewRequest("POST", ss.httpHost+apiEndpoint, bytes.NewBuffer(jsonData))
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := ss.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(response.Body)

	body, _ := io.ReadAll(response.Body)
	var searchResult = SearchResult{}
	err = json2.Unmarshal(body, &searchResult)
	if err != nil {
		panic(err)
	}

	return searchResult

}

func (ss *SoulseekService) GetSearchResult(query string) SearchResult {
	apiEndpoint := "/api/v0/searches/"

	var jsonData = []byte(`{
		"searchText": "` + query + `"
	}`)
	request, err := http.NewRequest("GET", ss.httpHost+apiEndpoint+query+"?includeResponses=true", bytes.NewBuffer(jsonData))
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := ss.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(response.Body)

	body, _ := io.ReadAll(response.Body)
	var searchResult = SearchResult{}

	err = json2.Unmarshal(body, &searchResult)
	if err != nil {
		fmt.Println(body)
		panic(err)
	}

	return searchResult
}

func (ss SoulseekService) Transfer(username string, filename string, size int) string {
	apiEndpoint := "/api/v0/transfers/downloads/"

	apiEndpoint += url.PathEscape(username)

	// fmt.Printf("Walę na %s\n", ss.httpHost+apiEndpoint)

	jsonData := make(map[string]any, 0)
	jsonData["filename"] = filename
	jsonData["size"] = size

	jsonEncapsulated := []any{jsonData}

	jsonRaw, err := json.Marshal(jsonEncapsulated)
	if err != nil {
		panic(err)
	}

	// fmt.Printf(string(jsonRaw))
	request, err := http.NewRequest("POST", ss.httpHost+apiEndpoint, bytes.NewBuffer(jsonRaw))
	if err != nil {
		panic(err)
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	response, err := ss.httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(response.Body)

	_, err = io.ReadAll(response.Body)
	if err != nil {
		panic(err)
	}

	return username + filename
}
