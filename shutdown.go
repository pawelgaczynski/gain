// Copyright (c) 2023 Paweł Gaczyński
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
