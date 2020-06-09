package lookup

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"io"
	"log"
	"sync"
	"time"
)

type Result struct {
	IP   string
	FQDN string
}

type Lookup struct {
	domain string
	addr string
}

func New(domain, addr string) *Lookup {
	if domain == "" {
		panic("domain cannot be empty")
	}

	return &Lookup{domain: domain, addr: addr}
}

func (l *Lookup) Run(ctx context.Context, maxWorkers int, r io.Reader) ([]Result, []error) {
	s := newSource(r)
	var wg sync.WaitGroup
	gather := make(chan []Result)
	runErrors := make(chan error)
	done := make(chan struct{})

	var errorBag []error
	var results []Result

	for i := 0; i < maxWorkers; i++ {
		go Worker(i + 1, &wg, ctx, l, s.pipe(), gather, runErrors)
	}

	go s.Run()

	go func() {
		for {
			select {
			case r := <-gather:
				if r != nil && len(r) > 0 {
					results = append(results, r...)
				} else {
					log.Println("Gathered an empty result or null")
				}
			case <-ctx.Done():
				log.Println("Context is done. Closing channels")
				s.stop()
				return
			case <-done:
				log.Println("All done. Closing channels")
				s.stop()
				return
			case err := <-runErrors:
				errorBag = append(errorBag, err)
			}
		}
	}()

	wg.Wait()

	close(done)

	log.Println("RUN is done")
	return results, errorBag
}


func (l *Lookup) fetchARecordsFor(fqdn string) ([]string, error) {
	var msg dns.Msg
	var ips []string
	msg.SetQuestion(dns.Fqdn(fqdn), dns.TypeA)

	log.Printf("\nChecking %s for A records", fqdn)

	resp, err := dns.Exchange(&msg, l.addr)
	if err != nil {
		return ips, errors.Wrapf(err, "could not get fetchARecordsFor records for %s from %s", fqdn, l.addr)
	}

	if len(resp.Answer) < 1 {
		return ips, errors.Errorf("No answer for fqdn [%s] on address [%s] when looking for fetchARecordsFor records", fqdn, l.addr)
	}

	for i := range resp.Answer {
		if a, ok := resp.Answer[i].(*dns.A); ok {
			ips = append(ips, a.A.String())
		}
	}

	return ips, nil
}

func (l *Lookup) fetchCNAMERecordsFor(fqdn string) ([]string, error) {
	var msg dns.Msg
	var cnames []string
	msg.SetQuestion(dns.Fqdn(fqdn), dns.TypeCNAME)

	log.Printf("\nChecking %s for CNAME records", fqdn)
	time.Sleep(2 * time.Second)
	resp, err := dns.Exchange(&msg, l.addr)
	if err != nil {
		return cnames, errors.Wrapf(err, "could not get fetchCNAMERecordsFor records for fqdn [%s] from [%s]", fqdn, l.addr)
	}

	if len(resp.Answer) < 1 {
		return cnames, errors.Errorf("No answer for fqdn [%s] on address %s when looking for fetchCNAMERecordsFor records", fqdn, l.addr)
	}

	for i := range resp.Answer {
		if cname, ok := resp.Answer[i].(*dns.CNAME); ok {
			cnames = append(cnames, cname.Target)
		}
	}

	return cnames, nil
}

func (l *Lookup) CreateFQDN(subdomain string) string {
	return fmt.Sprintf("%s.%s", subdomain, l.domain)
}

func (l *Lookup) fetchResultsFor(subdomain string) ([]Result, error) {
	var results []Result
	var fqdn = l.CreateFQDN(subdomain)

	for {
		cnames, err := l.fetchCNAMERecordsFor(fqdn)
		if err == nil && len(cnames) > 0 {
			fqdn = cnames[0]
			continue
		}

		ips, err := l.fetchARecordsFor(fqdn)
		if err != nil {
			return results, err
		}

		for i := range ips {
			results = append(results, Result{IP: ips[i], FQDN: fqdn})
		}

		return results, nil
	}
}

func Worker(id int, wg *sync.WaitGroup, ctx context.Context, l *Lookup, subdomains <-chan string, gather chan []Result, errs chan error) {
	wg.Add(1)

	done := make(chan struct{})

	go func() {
		for subdomain := range subdomains {
			results, err := l.fetchResultsFor(subdomain)
			if err != nil {
				errs <- err
				continue
			}

			if len(results) > 0 {
				select {
				case gather <- results:
				default:
					log.Println("Sink is closed")
				}
			}
		}

		close(done)
	}()

	for {
		select {
			case <-ctx.Done():
				log.Printf("\nATTENTION!!! Worker %d is forced to exit", id)
				wg.Done()
				return
			case <-done:
				log.Printf("\nWorker %d is done", id)
				wg.Done()
				return
		}
	}
}
