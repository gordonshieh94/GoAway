package main

import (
	"github.com/gordonshieh94/GopherHole/api"
	"github.com/gordonshieh94/GopherHole/blocklist"
	"github.com/gordonshieh94/GopherHole/dns"
	"io/ioutil"
	"net/http"
	"strings"
)

func importBlocklistFromHTTP(db *blocklist.Blocklist) {
	for _, url := range db.GetBlocklists() {
		resp, err := http.Get(url)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		toBlock := strings.Split(string(body), "\n")
		for _, host := range toBlock {
			println(host)
			db.AddHost(host)
		}
	}
}

func main() {
	db := blocklist.GetDatabase()
	dnsActivityChan := make(chan []byte)
	go importBlocklistFromHTTP(db)
	println("server started")
	go dns.Server(db, dnsActivityChan)
	api.StartAPIServer(db, dnsActivityChan)
}
