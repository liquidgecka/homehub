// Copyright 2026 - Brady Catherman
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

package task

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	s := NewScheduler()

	var wg sync.WaitGroup
	wg.Add(2)

	var task1Counter int
	var task2Counter int

	s.AddTask(&Task{
		Name:         "task1",
		InitialDelay: 0,
		Interval:     10 * time.Millisecond,
		Task: func(ctx context.Context) {
			task1Counter++
			wg.Done()
		},
	})

	s.AddTask(&Task{
		Name:         "task2",
		InitialDelay: 5 * time.Millisecond,
		Interval:     10 * time.Millisecond,
		Task: func(ctx context.Context) {
			task2Counter++
			wg.Done()
		},
	})

	s.Start()

	// Wait for the first two executions.
	wg.Wait()

	// Stop the scheduler.
	s.Stop()

	if task1Counter < 1 {
		t.Errorf("task1 should have run at least once")
	}
	if task2Counter < 1 {
		t.Errorf("task2 should have run at least once")
	}
}
