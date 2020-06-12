package lookup

import (
	"log"
)

type Result struct {
	IPs  []string
	FQDN string
}

type sink struct {
	inCh    chan *Result
	errCh   chan error
	stopCh  chan struct{}
	results []*Result
	errors  []error
}

func newSink() *sink {
	return &sink{
		errCh:   make(chan error),
		inCh:    make(chan *Result),
		stopCh:  make(chan struct{}),
		results: make([]*Result, 0, 10),
		errors:  make([]error, 0),
	}
}

func (s *sink) start() {
	go func() {
		for {
			select {
			case r := <-s.inCh:
				s.results = append(s.results, r)
			case err := <-s.errCh:
				s.errors = append(s.errors, err)
			case <-s.stopCh:
				log.Println("Sink is stopping")
				return
			}
		}
	}()
}

func (s *sink) stop() {
	close(s.stopCh)
}

func (s *sink) consumeResult() chan<- *Result {
	return s.inCh
}

func (s *sink) consumeError() chan<- error {
	return s.errCh
}

func (s *sink) unwrap() ([]*Result, []error) {
	return s.results, s.errors
}
