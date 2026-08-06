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

// Package task provides a simple background task scheduler that allows
// for tasks to be executed in the background at a given interval.
package task

import (
	"context"
	"log"
	"sync"
	"time"
)

// Func is the function that will be executed by the task.
type Func func(ctx context.Context)

// Task defines a background task that will be managed by the scheduler.
type Task struct {
	// Name is the name of the task.
	Name string

	// InitialDelay is the amount of time to wait before the first
	// execution of the task.
	InitialDelay time.Duration

	// Interval is the amount of time to wait between executions of the
	// task.
	Interval time.Duration

	// Task is the function that will be executed.
	Task Func
}

// Scheduler is used to manage a set of background tasks.
type Scheduler struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	tasks  []*Task
}

// NewScheduler creates a new task scheduler.
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		ctx:    ctx,
		cancel: cancel,
	}
}

// AddTask adds a new task to the scheduler.
func (s *Scheduler) AddTask(task *Task) {
	s.tasks = append(s.tasks, task)
}

// Start starts all the tasks in the scheduler. This is a non-blocking
// call.
func (s *Scheduler) Start() {
	for _, task := range s.tasks {
		s.wg.Add(1)
		go s.runTask(task)
	}
}

// Stop stops all the tasks in the scheduler. This is a blocking call.
func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
}

// runTask runs a single task in a loop until the scheduler is stopped.
func (s *Scheduler) runTask(task *Task) {
	defer s.wg.Done()

	// Wait for the initial delay.
	select {
	case <-s.ctx.Done():
		return
	case <-time.After(task.InitialDelay):
	}

	// Run the task for the first time.
	log.Printf("Starting background task: %s", task.Name)
	task.Task(s.ctx)
	log.Printf("Background task finished: %s", task.Name)

	// Now run the task on a timer.
	ticker := time.NewTicker(task.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			log.Printf("Starting background task: %s", task.Name)
			task.Task(s.ctx)
			log.Printf("Background task finished: %s", task.Name)
		}
	}
}
