package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	elasticsearch "github.com/elastic/go-elasticsearch/v7"
)

// HitCounter - for unmarshaling search results
type HitCounter struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
	} `json:"hits"`
}

func main() {

	// Define config values and thresholds
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s: [options] [elastic hosts]\nHost format: http://host1:9200\n", os.Args[0])
		flag.PrintDefaults()
	}
	var jsonFile = flag.String("q", "", "JSON file holding the query")
	var esIndex = flag.String("i", "index", "Elasticsearch Index")
	var statusOutput = flag.String("o", "hits for elastic search.", "Custom phrasing for output")
	var username = flag.String("u", "", "Username")
	var password = flag.String("p", "", "Password")
	var apikey = flag.String("a", "", "API Token")
	var cloudid = flag.String("I", "", "CloudID")
	var certificate = flag.String("ca", "", "CA Certificate file")
	var counterFile = flag.String("cf", "", "Counter file for persistent errors")
	var event = flag.Bool("e", false, "Run as event handler to remove counter file")
	var currentStatus = flag.Int("s", 0, "Current status according to Icinga. Remove counter file if 'event' and 'OK'")
	var critVal = flag.Int64("c", 0, "Critical number of hits")
	var warnVal = flag.Int64("w", 0, "Warning number of hits")
	flag.Parse()
	var hosts = flag.Args()

	// Purge counter file and quit if running as an event
	if *event && *currentStatus == 0 {
		os.Remove(*counterFile)
		os.Exit(0)
	}

	// Prepare config object for client
	var cfg = elasticsearch.Config{}
	cfg.Addresses = hosts
	if *username != "" {
		cfg.Username = *username
	}
	if *password != "" {
		cfg.Password = *password
	}
	if *apikey != "" {
		cfg.APIKey = *apikey
	}
	if *cloudid != "" {
		cfg.CloudID = *cloudid
	}
	if *certificate != "" {
		cert, _ := ioutil.ReadFile(*certificate)
		cfg.CACert = cert
	}

	// Initialize client
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		fmt.Println("UNKNOWN: Could not initialize elasticsearch client.")
		fmt.Print(err)
		os.Exit(3)
	}

	// Load JSON query
	// payload, err := os.Open(*jsonFile)
	fileContent, err := ioutil.ReadFile(*jsonFile)
	if err != nil {
		fmt.Println("UNKNOWN: Could not open query file.")
		fmt.Print(err)
		os.Exit(3)
	}

  // Get json file content and expand env variables
	text := string(fileContent)
	textExp := os.ExpandEnv(text)
	// fmt.Println(textExp)
	payload := strings.NewReader(textExp)

	// Execute search
	res, err := es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(*esIndex),
		es.Search.WithBody(payload),
		es.Search.WithTrackTotalHits(true),
	)
	if err != nil {
		fmt.Println("UNKNOWN: Could not retrieve data from elasticsearch.")
		fmt.Print(err)
		os.Exit(3)
	}
	defer res.Body.Close()

	// Unmarshal response
	body, _ := ioutil.ReadAll(res.Body)
	var counter HitCounter
	err = json.Unmarshal(body, &counter)
	if err != nil {
		fmt.Println("UNKNOWN: Could not unmarshal search data")
		fmt.Print(err)
		os.Exit(3)
	}

	// Track hits and if a counter is specified, add together
	hits := counter.Hits.Total.Value
	if *counterFile != "" {
		if _, err := os.Stat(*counterFile); !os.IsNotExist(err) {
			var priorHits int64
			content, err := ioutil.ReadFile(*counterFile)
			if err != nil {
				fmt.Printf("UNKNOWN: could not open counter file")
				os.Exit(3)
			}
			readbuf := bytes.NewReader(content)
			err = binary.Read(readbuf, binary.LittleEndian, &priorHits)
			if err != nil {
				fmt.Print(err)
			}
			hits += priorHits
		}
		writebuf := new(bytes.Buffer)
		err = binary.Write(writebuf, binary.LittleEndian, hits)
		if err != nil {
			fmt.Print(err)
		}
		err = ioutil.WriteFile(*counterFile, writebuf.Bytes(), 0644)
		if err != nil {
			fmt.Print(err)
		}
	}

	// Evaluate and alert
	if hits >= *critVal {
		fmt.Printf("CRITICAL: %d %s|hits=%d;%d;%d;;", hits, *statusOutput, hits, *warnVal, *critVal)
		os.Exit(2)
	} else if hits >= *warnVal {
		fmt.Printf("WARNING: %d %s|hits=%d;%d;%d;;", hits, *statusOutput, hits, *warnVal, *critVal)
		os.Exit(1)
	} else {
		fmt.Printf("OK: %d %s|hits=%d;%d;%d;;", hits, *statusOutput, hits, *warnVal, *critVal)
		os.Exit(0)
	}
}
