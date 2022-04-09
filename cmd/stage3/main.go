package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

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

	f, err := os.Create("dump.csv")
	handle(err, "create dump.csv")
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write([]string{"hostname", "scheme", "dns_ns", "first_ns", "all_ns", "all_byte", "Server", "X-Served-By", "Content-Type"})

	rows, err := db.Query(`SELECT hostname, scheme, dns_ns, first_ns, all_ns, all_byte, headers_json, body FROM stage2;`)
	handle(err, "query 1")

	for rows.Next() {
		var hostname, scheme, headersJSON, body string
		var dnsNs, firstNs, allNs, allBytes int64
		err = rows.Scan(&hostname, &scheme, &dnsNs, &firstNs, &allNs, &allBytes, &headersJSON, &body)
		handle(err, "scan 1")
		m := make(map[string][]string)
		err = json.Unmarshal([]byte(headersJSON), &m)
		handle(err, "unmarshal header json")

		hm := http.Header(m)

		w.Write([]string{
			hostname, scheme,
			strconv.FormatInt(dnsNs, 10), strconv.FormatInt(firstNs, 10),
			strconv.FormatInt(allNs, 10), strconv.FormatInt(allBytes, 10),
			hm.Get("server"), hm.Get("X-Served-By"), hm.Get("Content-Type"),
		})
	}
}
