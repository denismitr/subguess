package lookup

import (
	"github.com/pkg/errors"
	"sync"
)

type sink struct {
	mu     sync.Mutex
	rs     []Result
	errs   []error
	status status
}

func newSink() *sink {
	return &sink{
		rs:     make([]Result, 0, 5),
		errs:   make([]error, 0),
		status: notStarted,
	}
}

func (s *sink) start() *sink {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = running
	return s
}

func (s *sink) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = stopped
}

func (s *sink) results(r []Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == stopped || s.status == notStarted {
		return errors.Errorf("Sink is not in running state")
	}

	for i := range r {
		s.rs = append(s.rs, r[i])
	}

	return nil
}

func (s *sink) error(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == stopped || s.status == notStarted {
		return
	}

	s.errs = append(s.errs, err)
}

func (s *sink) unwrap() ([]Result, []error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rs, s.errs
}

