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

package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gcalendar "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

func newMockCalendarService(
	t *testing.T, handler http.Handler,
) (*gcalendar.Service, *httptest.Server) {
	ts := httptest.NewServer(handler)

	srv, err := gcalendar.NewService(
		context.Background(),
		option.WithEndpoint(ts.URL),
		option.WithHTTPClient(ts.Client()),
	)
	if err != nil {
		t.Fatalf("unable to create calendar service: %v", err)
	}
	return srv, ts
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
