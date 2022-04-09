package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

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

	_, err = db.Exec(`DROP TABLE IF EXISTS stage2;`)
	handle(err, "drop table stage2 ")
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS stage2 (
        hostname TEXT,
        scheme TEXT,
        tls_ok INTEGER,
        dns_ns INTEGER,
        addr TEXT,
        first_ns INTEGER,
        all_ns INTEGER,
        all_byte INTEGER,
        headers_json TEXT,
        body TEXT,
        PRIMARY KEY (hostname, scheme)
);`)
	handle(err, "create table stageqli")

	rows, err := db.Query(`SELECT hostname FROM stage1;`)
	handle(err, "query stage1")
	var hosts []string
	for rows.Next() {
		var host string
		err = rows.Scan(&host)
		handle(err, "stage1 scan")
		hosts = append(hosts, host)
	}
	err = rows.Err()
	handle(err, "scan stage1")

	workChan := make(chan string)
	handle(err, "get work db conn")
	go func() {
		defer close(workChan)
		for _, host := range hosts {
			workChan <- host
		}
	}()

	var wg sync.WaitGroup
	resultChan := make(chan Result)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go worker(&wg, workChan, resultChan)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	handle(err, "db result conn")
	for r := range resultChan {
		_, err = db.ExecContext(context.TODO(), `INSERT INTO stage2
(hostname, scheme, tls_ok, dns_ns, addr, first_ns, all_ns, all_byte, headers_json, body)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`, r.Hostname, r.Scheme, r.TLS, r.DNS, r.Addr, r.First, r.All, r.Size, r.Headers, r.Body)
		handle(err, "insert stage2")
	}
}

type Result struct {
	Hostname string
	Scheme   string
	TLS      int
	DNS      time.Duration
	Addr     string
	First    time.Duration
	All      time.Duration
	Size     int
	Headers  string
	Body     string
}

func worker(wg *sync.WaitGroup, work chan string, results chan Result) {
	defer wg.Done()
	tr := http.DefaultTransport.(*http.Transport).Clone()

	for host := range work {
		r, err := do(tr, "http", host)
		if err != nil {
			log.Println("http", host, err)
		} else {
			results <- r
		}
		r, err = do(tr, "https", host)
		if err != nil {
			log.Println("https", host, err)
		} else {
			results <- r
		}
	}
}

func do(tr *http.Transport, scheme, host string) (Result, error) {
	if strings.HasSuffix(host, ".eth") {
		host = host + ".xyz"
	}

	var verifiedTLS int
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(verifiedChains) > 0 {
				verifiedTLS = 1
			}
			return nil
		},
	}

	client := http.Client{
		Transport: tr,
	}

	r := Result{
		Hostname: host,
		Scheme:   scheme,
	}

	var dnsStart time.Time
	ctx := context.Background()
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		DNSStart: func(di httptrace.DNSStartInfo) {
			dnsStart = time.Now()
		},
		DNSDone: func(di httptrace.DNSDoneInfo) {
			r.DNS = time.Since(dnsStart)
		},
		GotConn: func(gci httptrace.GotConnInfo) {
			r.Addr = gci.Conn.RemoteAddr().String()
		},
		GotFirstResponseByte: func() {
			r.First = time.Since(dnsStart)
		},
	})
	u := scheme + "://" + host + "/"
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create url %s: %w", u, err)
	}
	req.Header.Set("user-agent", "go.seankhliao.com/hn-sistes-2022-04")

	t0 := time.Now()
	res, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("do request %s: %w", u, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	dur := time.Since(t0)
	if err != nil {
		return Result{}, fmt.Errorf("read body %s: %w", u, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return Result{}, fmt.Errorf("request %s: %s", u, res.Status)
	}

	headerJSON, err := json.Marshal(res.Header)
	if err != nil {
		return Result{}, fmt.Errorf("marshal headers: %w", err)
	}

	r.All = dur
	r.Size = len(body)
	r.Headers = string(headerJSON)
	r.Body = string(body)
	r.TLS = verifiedTLS
	return r, nil
}
