package main

import (
	"database/sql"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/exp/slices"
	_ "modernc.org/sqlite"
)

func handle(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, ":", err)
	}
}

var urlRe = regexp.MustCompile(`[Hh][Tt]{2}[Pp](s)?://[a-zA-Z0-9-/=?&%\.]+`) // url-like

func main() {
	db, err := sql.Open("sqlite", "file:db.sqlite")
	handle(err, "open db")
	defer db.Close()

	files, err := filepath.Glob("src/hn*.html")
	handle(err, "glob")

	var hosts []string
	for _, file := range files {
		raw, err := os.ReadFile(file)
		handle(err, "read "+file)
		us := urlRe.FindAllString(string(raw), -1)
		for _, u := range us {
			if strings.HasSuffix(u, "...") { // ignore truncated urls
				continue
			}
			u = strings.ReplaceAll(u, "///", "//") // bad urls "//"
			u = strings.ToLower(u)                 // some people use caps
			pu, err := url.Parse(u)
			handle(err, "parse url "+u)
			if pu.Path == "" {
				pu.Path = "/"
			}
			if pu.Host == "news.ycombinator.com" {
				continue
			}
			hosts = append(hosts, pu.Host)
		}
	}
	slices.Sort(hosts)
	hosts = slices.Compact(hosts) // uniq only

	_, err = db.Exec(`DROP TABLE IF EXISTS stage1;`)
	handle(err, "drop table stage1")
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS stage1 (hostname TEXT PRIMARY KEY);`)
	handle(err, "create table stage1")

	for _, host := range hosts {
		_, err = db.Exec(`INSERT INTO stage1 (hostname) VALUES (?);`, host)
		handle(err, "insert")
	}
}
