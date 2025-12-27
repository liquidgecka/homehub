// Copyright 2026 - Brady Catherman
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package activity

import (
	"sync"
	"testing"
)

func TestActivityReset(t *testing.T) {
	// Store the original function.
	original := ResetTimer

	// When the test is done, restore the original function.
	t.Cleanup(func() {
		ResetTimer = original
	})

	// Create a wait group to wait for the reset timer to be called.
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Set our own function that will be called when the reset timer
	// is called.
	ResetTimer = func() {
		wg.Done()
	}

	// In a go routine, call the reset timer.
	go ResetTimer()

	// Wait for the reset timer to be called.
	wg.Wait()
}
