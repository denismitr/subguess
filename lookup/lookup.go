package lookup

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"io"
	"log"
	"sync"
)

type Result struct {
	IP   string
	FQDN string
}

type Lookup struct {
	domain string
	addr string
}

type GatherFunc func()

func (l *Lookup) gatherFunc(streamer Streamer, sink *sink) GatherFunc {
	return func() {
		for subdomain := range streamer.Stream() {
			results, err := l.fetchResultsFor(subdomain)
			if err != nil {
				sink.error(err)
				continue
			}

			if len(results) > 0 {
				if err := sink.results(results); err != nil {
					log.Println(err)
				}
			}
		}
	}
}

func New(domain, addr string) *Lookup {
	if domain == "" {
		panic("domain cannot be empty")
	}

	return &Lookup{domain: domain, addr: addr}
}

func (l *Lookup) Run(ctx context.Context, maxWorkers int, r io.Reader) ([]Result, []error) {
	var wg sync.WaitGroup

	src := newSource(r)
	sink := newSink().start()
	done := make(chan struct{})

	for i := 0; i < maxWorkers; i++ {
		go Worker(i + 1, &wg, ctx, l.gatherFunc(src, sink))
	}

	go src.start()

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Context is done. Closing channels")
				sink.error(ctx.Err())
				src.stop()
				sink.stop()
				return
			case <-done:
				log.Println("All done. Closing channels")
				src.stop()
				sink.stop()
				return
			}
		}
	}()

	wg.Wait()

	close(done)

	log.Println("RUN is done")
	return sink.unwrap()
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

func Worker(id int, wg *sync.WaitGroup, ctx context.Context, gf GatherFunc) {
	wg.Add(1)

	done := make(chan struct{})

	go func() {
		gf()
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
