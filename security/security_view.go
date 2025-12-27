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

package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/dialogs"
)

type frigateCamera struct {
	*config.CameraConfig
	token string
}

func (f *frigateCamera) login() error {
	u, err := url.Parse(f.URL)
	if err != nil {
		return err
	}
	loginURL := fmt.Sprintf("%s://%s/api/login", u.Scheme, u.Host)

	body, err := json.Marshal(map[string]string{
		"user":     f.Username,
		"password": f.Password,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(loginURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed with status code %d", resp.StatusCode)
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "frigate_token" {
			f.token = cookie.Value
			return nil
		}
	}

	return fmt.Errorf("frigate_token cookie not found in login response")
}

type securityView struct {
	content fyne.CanvasObject
	win     fyne.Window
	stops   []chan struct{}
	cameras []*frigateCamera
}

func (s *securityView) GetContent() fyne.CanvasObject {
	return s.content
}

func (s *securityView) Stop() {
	for _, stop := range s.stops {
		close(stop)
	}
}

func New(win fyne.Window) *securityView {
	s := &securityView{
		win: win,
	}

	cameras := config.GetConfig().Security.Camera
	if len(cameras) == 0 {
		s.content = widget.NewLabel("No cameras configured.")
		return s
	}

	s.cameras = make([]*frigateCamera, len(cameras))
	for i := range cameras {
		s.cameras[i] = &frigateCamera{CameraConfig: &cameras[i]}
	}

	grid := container.NewGridWithColumns(2)
	for _, cam := range s.cameras {
		if cam.Type == "frigate" {
			if err := cam.login(); err != nil {
				log.Printf("Error logging into frigate camera %s: %v", cam.Name, err)
			}
		}
		grid.Add(s.createCameraView(cam))
	}
	s.content = grid

	return s
}

func (s *securityView) createCameraView(camera *frigateCamera) fyne.CanvasObject {
	img := &canvas.Image{
		FillMode: canvas.ImageFillContain,
	}
	img.SetMinSize(fyne.NewSize(320, 240))

	stop := make(chan struct{})
	s.stops = append(s.stops, stop)
	go s.fetchImage(camera, img, stop)

	card := widget.NewCard(camera.Name, "", img)

	return container.NewMax(card, &tappable{
		onTap: func() {
			if img.Resource != nil {
				dialogs.ShowImageDialog(s.win, camera.Name, img.Resource)
			}
		},
	})
}

func (s *securityView) fetchImage(camera *frigateCamera, img *canvas.Image, stop chan struct{}) {
	d, err := time.ParseDuration(camera.Refresh)
	if err != nil || camera.Refresh == "" {
		if camera.Refresh != "" {
			log.Printf("Error parsing refresh duration '%s': %v. Defaulting to 200ms.", camera.Refresh, err)
		}
		d = 200 * time.Millisecond
	}

	fetchQueue := make(chan bool, 1)
	fetchQueue <- true // Start with a fetch

	go func() {
		ticker := time.NewTicker(d)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				select {
				case fetchQueue <- true:
				default:
					// a fetch is already in progress, skip this one
				}
			}
		}
	}()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	backoff := 100 * time.Millisecond
	const maxBackoff = 1 * time.Second

	for {
		select {
		case <-stop:
			return
		case <-fetchQueue:
		}

		req, err := http.NewRequest("GET", camera.URL, nil)
		if err != nil {
			log.Printf("Error creating request for camera %s: %v", camera.Name, err)
			time.Sleep(backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		if camera.Type == "frigate" {
			req.Header.Set("Authorization", "Bearer "+camera.token)
		} else if camera.Username != "" && camera.Password != "" {
			req.SetBasicAuth(camera.Username, camera.Password)
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error fetching image for camera %s: %v", camera.Name, err)
			time.Sleep(backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		if resp.StatusCode == http.StatusUnauthorized && camera.Type == "frigate" {
			log.Printf("Token expired for camera %s, refreshing...", camera.Name)
			if err := camera.login(); err != nil {
				log.Printf("Error refreshing token for camera %s: %v", camera.Name, err)
				resp.Body.Close()
				time.Sleep(backoff)
				backoff = min(backoff*2, maxBackoff)
				continue
			}
			// Retry the request
			resp.Body.Close()
			fetchQueue <- true
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Printf("Error fetching image for camera %s: status code %d", camera.Name, resp.StatusCode)
			resp.Body.Close()
			time.Sleep(backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		// Success
		backoff = 100 * time.Millisecond

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("Error reading image for camera %s: %v", camera.Name, err)
			time.Sleep(backoff)
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		fyne.Do(func() {
			img.Resource = fyne.NewStaticResource(fmt.Sprintf("%s.jpg", camera.Name), data)
			img.Refresh()
		})
	}
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

type tappable struct {
	widget.BaseWidget
	onTap func()
}

func (t *tappable) CreateRenderer() fyne.WidgetRenderer {
	t.ExtendBaseWidget(t)
	return &tappableRenderer{t}
}

func (t *tappable) Tapped(_ *fyne.PointEvent) {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	t.onTap()
}

func (t *tappable) TappedSecondary(_ *fyne.PointEvent) {
}

func (t *tappable) Cursor() desktop.Cursor {
	return desktop.PointerCursor
}

type tappableRenderer struct {
	t *tappable
}

func (r *tappableRenderer) Destroy() {}

func (r *tappableRenderer) Layout(size fyne.Size) {
}

func (r *tappableRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *tappableRenderer) Objects() []fyne.CanvasObject {
	return nil
}

func (r *tappableRenderer) Refresh() {
}
