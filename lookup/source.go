package lookup

import (
	"bufio"
	"io"
	"log"
	"sync"
)

type status int

const (
	notStarted status = iota
	running
	stopped
)

type Streamer interface {
	Stream() <-chan string
}

type source struct {
	scanner *bufio.Scanner

	mu         sync.Mutex
	status     status
	subdomains chan string
}

func newSource(r io.Reader) *source {
	return &source{
		scanner: bufio.NewScanner(r),
		status: notStarted,
		subdomains: make(chan string),
	}
}

func (s *source) Stream() <-chan string {
	return s.subdomains
}

func (s *source) start() {
	s.mu.Lock()
	s.status = running
	s.mu.Unlock()

	for s.scanner.Scan() {
		s.subdomains <- s.scanner.Text()
	}

	s.stop()
}

func (s *source) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == running {
		close(s.subdomains)
		s.status = stopped
		log.Println("Source is stopped!!!!")
	}
}
