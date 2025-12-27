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

package background

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/liquidgecka/homehub/task"
)

func TestManager(t *testing.T) {
	m := NewManager()

	var wg sync.WaitGroup
	wg.Add(1)

	m.scheduler.AddTask(&task.Task{
		Name:         "test task",
		InitialDelay: 0,
		Interval:     10 * time.Millisecond,
		Task: func(ctx context.Context) {
			wg.Done()
		},
	})

	m.Start()
	wg.Wait()
	m.Stop()
}
