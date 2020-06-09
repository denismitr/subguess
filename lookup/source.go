package lookup

import (
	"bufio"
	"io"
	"log"
	"sync"
)

type sourceStatus int

const (
	notStarted sourceStatus = iota
	running
	stopped
)

type source struct {
	scanner *bufio.Scanner

	mu sync.Mutex
	status sourceStatus
	subdomains chan string
}

func newSource(r io.Reader) *source {
	return &source{
		scanner: bufio.NewScanner(r),
		status: notStarted,
		subdomains: make(chan string),
	}
}

func (s *source) pipe() <-chan string {
	return s.subdomains
}

func (s *source) Run() {
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
