package lookup

import (
	"bufio"
	"context"
	"fmt"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"io"
	"log"
	"sync"
)

type Result struct {
	IP string
	Hostname string
}

func Run(ctx context.Context, maxWorkers int, r io.Reader, domain string, addr string) ([]Result, error) {
	scanner := bufio.NewScanner(r)
	var wg sync.WaitGroup
	domains := make(chan string)
	gather := make(chan []Result)
	errs := make(chan error)

	var results []Result

	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go Worker(&wg, domains, addr, gather, errs)
	}

	go func() {
		for scanner.Scan() {
			domains <- fmt.Sprintf("%s.%s", scanner.Text(), domain)
		}

		close(domains)
	}()

	for {
		select {
			case r := <-gather:
				if r != nil && len(r) > 0 {
					results = append(results, r...)
				} else {
					log.Println("Gathered an empty result or null")
				}
			case <-ctx.Done():
				log.Println("Context is done")
				close(domains)
				close(gather)
				return results, nil
			case err := <-errs:
				log.Println(err)
		}
	}
}

func A(domain, addr string) ([]string, error) {
	var msg dns.Msg
	var ips []string
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	resp, err := dns.Exchange(&msg, addr)
	if err != nil {
		return ips, errors.Wrapf(err, "could not get A records for %s from %s", domain, addr)
	}

	if len(resp.Answer) < 1 {
		return ips, errors.Errorf("No answer for %s on address %s when looking for A records", domain, addr)
	}

	for i := range resp.Answer {
		if a, ok := resp.Answer[i].(*dns.A); ok {
			ips = append(ips, a.A.String())
		}
	}

	return ips, nil
}

func CNAME(domain, addr string) ([]string, error) {
	var msg dns.Msg
	var cnames []string
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeCNAME)
	resp, err := dns.Exchange(&msg, addr)
	if err != nil {
		return cnames, errors.Wrapf(err, "could not get CNAME records for domain %s from %s", domain, addr)
	}

	if len(resp.Answer) < 1 {
		return cnames, errors.Errorf("No answer for domain %s on address %s when looking for CNAME records", domain, addr)
	}

	for i := range resp.Answer {
		if cname, ok := resp.Answer[i].(*dns.CNAME); ok {
			cnames = append(cnames, cname.Target)
		}
	}

	return cnames, nil
}

func fetch(domain, addr string) ([]Result, error) {
	var results []Result
	var hostname = domain

	for {
		cnames, err := CNAME(hostname, addr)
		if err != nil {
			log.Println(err)
		} else if len(cnames) > 0 {
			hostname = cnames[0]
			continue
		}

		ips, err := A(domain, addr)
		if err != nil {
			return results, err
		}

		for i := range ips {
			results = append(results, Result{IP: ips[i], Hostname: hostname})
		}
	}
}

func Worker(wg *sync.WaitGroup, domains chan string, addr string, gather chan []Result, errs chan error) {
	for domain := range domains {
		result, err := fetch(domain, addr)
		if err != nil {
			errs <- err
			continue
		}

		if len(result) > 0 {
			gather <- result
		}
	}

	wg.Done()
}
