package main

import (
	"flag"
	"fmt"
	"github.com/denismitr/subguess/lookup"
	"golang.org/x/net/context"
	"log"
	"os"
	"time"
)

func main() {
	var domain = flag.String("domain", "", "Domain to guess subdomains on")
	var workers = flag.Int("workers", 100, "Number of workers to run")
	var timeout = flag.Int("timeout", 5, "Max timeout")
	var source = flag.String("source", "", "Source file to read suggestions from")
	var addr = flag.String("server", "8.8.8.8:53", "DNS server address to use")

	flag.Parse()

	if *domain == "" || *source == "" {
		fmt.Println("Error: -domain and -source are required parameters")
		os.Exit(1)
	}

	f, err := os.Open(*source)
	if err != nil {
		log.Fatalf("No %s source file found", *source)
	}

	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout) * time.Second)
	defer cancel()

	l := lookup.New(*domain, *addr)
	results, errorBag := l.Run(ctx, *workers, f)

	if len(results) > 0 {
		fmt.Printf("\nFound %d results\n", len(results))

		for i := range results {
			fmt.Printf("\nFQDN:\t%s\tIP:\t%s\n", results[i].FQDN, results[i].IP)
		}
	} else {
		fmt.Printf("\nNo results found for %s at %s", *domain, *addr)

		for i := range errorBag {
			log.Printf("\nError: %s", errorBag[i].Error())
		}
	}
}
