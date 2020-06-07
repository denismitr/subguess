package main

import (
	"flag"
	"fmt"
	"github.com/denismitr/subguess/lookup"
	"golang.org/x/net/context"
	"text/tabwriter"
	"os"
	"time"
)

func main() {
	var domain = flag.String("domain", "", "Domain to guess subdomains on")
	var workers = flag.Int("workers", 10, "Number of workers to run")
	var source = flag.String("source", "", "Source file to read suggestions from")
	var addr = flag.String("server", "8.8.8.8:53", "DNS server address to use")

	flag.Parse()

	if *domain == "" || *source == "" {
		fmt.Println("Error: -domain and -source are required parameters")
		os.Exit(1)
	}

	f, err := os.Open(*source)
	if err != nil {
		panic(err)
	}

	defer f.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Second)
	defer cancel()

	results, err := lookup.Run(ctx, *workers, f, *domain, *addr)
	if err != nil {
		panic(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 4, 0, 0)

	for i := range results {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", results[i].Hostname, results[i].IP)
	}
}
