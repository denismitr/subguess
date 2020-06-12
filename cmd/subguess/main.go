package main

import (
	"flag"
	"fmt"
	"github.com/denismitr/subguess/lookup"
	"github.com/jedib0t/go-pretty/v6/table"
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
		drawResultsTable(results)
	} else {
		drawErrorsTable(errorBag)
	}
}

func drawResultsTable(results []*lookup.Result) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "FQDB", "##", "IP address"})

	var totalIPs = make(map[string]bool)
	for i := range results {
		for j := range results[i].IPs {
			totalIPs[results[i].IPs[j]] = true
			if j == 0 {
				t.AppendSeparator()
				t.AppendRow(table.Row{
					fmt.Sprintf("%d", i + 1),
					results[i].FQDN,
					fmt.Sprintf("%d", j + 1),
					results[i].IPs[j],
				})
			} else {
				t.AppendRow(table.Row{
					"",
					"",
					fmt.Sprintf("%d", j + 1),
					results[i].IPs[j],
				})
			}
		}
	}

	t.AppendFooter(table.Row{"Total domain names", len(results), "Total IPs", len(totalIPs)})

	t.Render()
}

func drawErrorsTable(errs []error) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Error", "Level"})

	for i := range errs {
		t.AppendRow(table.Row{fmt.Sprintf("%d", i + 1), errs[i].Error(), "UNKNOWN"}) // fixme
	}

	t.AppendFooter(table.Row{"", "Count", len(errs)})

	t.Render()
}
