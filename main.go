package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type SearchEntry struct {
	EndedAt         time.Time `json:"endedAt"`
	FileCount       int       `json:"fileCount"`
	ID              string    `json:"id"`
	IsComplete      bool      `json:"isComplete"`
	LockedFileCount int       `json:"lockedFileCount"`
	ResponseCount   int       `json:"responseCount"`
	Responses       []any     `json:"responses"`
	SearchText      string    `json:"searchText"`
	StartedAt       time.Time `json:"startedAt"`
	State           string    `json:"state"`
	Token           int       `json:"token"`
}

func main() {
	client := &http.Client{}

	resp, err := http.Get("http://192.168.88.6:5030/api/v0/searches")
	if err != nil {
		log.Fatalln(err)
	}

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var m []SearchEntry
	err = json.Unmarshal(raw, &m)

	for _, searchEntry := range m {
		fmt.Println(searchEntry.ID, searchEntry.SearchText, searchEntry.State)

		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://192.168.88.6:5030/api/v0/searches/%s", searchEntry.ID), nil)
		if err != nil {
			log.Fatalln(err)
		}

		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)
		}

		// print the response body
		fmt.Println(string(body))
	}

	//fmt.Println(m[0].SearchText)

}
