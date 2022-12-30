package gain

import (
	"sync"
)

type shutdowner struct {
	shutdowning bool
	inProgress  bool
	wg          sync.WaitGroup
}

func (s *shutdowner) markShutdownInProgress() {
	s.inProgress = true
}

func (s *shutdowner) needToShutdown() bool {
	return s.shutdowning && !s.inProgress
}

func (s *shutdowner) notifyFinish() {
	if s.shutdowning {
		s.wg.Done()
	}
}

func (s *shutdowner) shutdown() {
	s.shutdowning = true
	s.wg.Add(1)
	s.wg.Wait()
}

func newShutdowner() *shutdowner {
	return &shutdowner{}
}
