package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	_ "modernc.org/sqlite"
)

func handle(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, ":", err)
	}
}

func main() {
	db, err := sql.Open("sqlite", "file:db.sqlite")
	handle(err, "open db")
	defer db.Close()

	rows, err := db.Query(`SELECT all_byte, headers_json FROM stage2 WHERE scheme = 'https';`)
	handle(err, "query 1")

	var sizes []int
	headerCount := make(map[string]int)
	for rows.Next() {
		var size int
		var rawHeader string
		err = rows.Scan(&size, &rawHeader)
		handle(err, "scan 1")
		sizes = append(sizes, size)
		var header map[string][]string
		err = json.Unmarshal([]byte(rawHeader), &header)
		handle(err, "unmarshal header")
		for key := range header {
			headerCount[key]++
		}
	}

	var headerOut []struct {
		string
		int
	}
	for k, v := range headerCount {
		headerOut = append(headerOut, struct {
			string
			int
		}{k, v})
	}
	sort.Slice(headerOut, func(i, j int) bool { return headerOut[i].int < headerOut[j].int })
	for _, h := range headerOut {
		fmt.Println(h.int, h.string)
	}

	var totalSize int
	for _, s := range sizes {
		totalSize += s
	}

	fmt.Println("avg size", totalSize/len(sizes))
}
