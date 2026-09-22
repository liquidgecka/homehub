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

package celebrations

import (
	"context"
	"image/color"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/ui"
)

// DisplayDuration defines how long a celebration overlay stays visible on
// screen.
const DisplayDuration = 25 * time.Second

// loadIconResource safely loads an SVG icon from disk with fallback.
func loadIconResource(iconFileName string) fyne.Resource {
	iconPath := ui.GetIconPath(iconFileName)
	data, err := os.ReadFile(iconPath)
	if err == nil && len(data) > 0 {
		return fyne.NewStaticResource(iconFileName, data)
	}
	// Fallback to relative icons/ directories if not at configured path
	for _, dir := range []string{"icons", "../icons", "../../icons"} {
		data, err = os.ReadFile(filepath.Join(dir, iconFileName))
		if err == nil && len(data) > 0 {
			return fyne.NewStaticResource(iconFileName, data)
		}
	}
	return theme.InfoIcon()
}

// GetIconForType returns the matching festive icon resource for a celebration
// type.
func GetIconForType(cType string) fyne.Resource {
	switch strings.ToLower(strings.TrimSpace(cType)) {
	case "birthday":
		return loadIconResource("balloons.svg")
	case "anniversary":
		return loadIconResource("rings.svg")
	case "graduation":
		return loadIconResource("graduation.svg")
	case "school", "first_day_of_school":
		return loadIconResource("school.svg")
	case "party", "congratulations", "holiday":
		return loadIconResource("party.svg")
	default:
		return loadIconResource("party.svg")
	}
}

// GetBannerText returns an appropriate festive banner headline.
func GetBannerText(c database.Celebration) string {
	switch strings.ToLower(strings.TrimSpace(c.Type)) {
	case "birthday":
		return "🎈 HAPPY BIRTHDAY! 🎈"
	case "anniversary":
		return "💍 HAPPY ANNIVERSARY! 💍"
	case "graduation":
		return "🎓 CONGRATULATIONS GRADUATE! 🎓"
	case "school", "first_day_of_school":
		return "🎒 FIRST DAY OF SCHOOL! 🎒"
	case "party":
		return "🎉 CELEBRATION! 🎉"
	case "holiday":
		return "🌟 HAPPY HOLIDAYS! 🌟"
	default:
		return "🎉 CELEBRATION 🎉"
	}
}

func randFloat(min, max float32) float32 {
	if max <= min {
		return min
	}
	return min + rand.Float32()*(max-min)
}

type balloonItem struct {
	img      *canvas.Image
	w, h     float32
	baseX    float32
	y        float32
	vy       float32
	swayAmp  float32
	swayFreq float32
	phase    float32
}

type ringsItem struct {
	img     *canvas.Image
	w, h    float32
	x, y    float32
	vx      float32
	baseY   float32
	bobAmp  float32
	bobFreq float32
}

type sparkleItem struct {
	img     *canvas.Image
	w, h    float32
	relX    float32
	relY    float32
	orbFreq float32
	phase   float32
}

type confettiItem struct {
	rect     *canvas.Rectangle
	baseX    float32
	y        float32
	vy       float32
	swayAmp  float32
	swayFreq float32
	phase    float32
}

func getScreenBounds(root fyne.CanvasObject) (float32, float32) {
	sz := root.Size()
	w := sz.Width
	h := sz.Height
	if w < 200 {
		w = 1024
	}
	if h < 200 {
		h = 600
	}
	return w, h
}

func startBalloonsAnimation(
	ctx context.Context,
	canvasContainer *fyne.Container,
	root fyne.CanvasObject,
) {
	balloonIcons := []string{
		"balloon_red.svg",
		"balloon_blue.svg",
		"balloon_gold.svg",
		"balloon_purple.svg",
		"balloon_green.svg",
		"balloons.svg",
	}

	w, h := getScreenBounds(root)
	count := 9
	balloons := make([]balloonItem, count)

	for i := 0; i < count; i++ {
		iconName := balloonIcons[i%len(balloonIcons)]
		img := canvas.NewImageFromResource(loadIconResource(iconName))
		img.FillMode = canvas.ImageFillContain

		bw := randFloat(55, 80)
		bh := bw * 1.5
		if iconName == "balloons.svg" {
			bh = bw
		}
		img.Resize(fyne.NewSize(bw, bh))

		baseX := randFloat(20, w-bw-20)
		initY := randFloat(h*0.1, h+250)
		vy := randFloat(65, 110)
		swayAmp := randFloat(15, 32)
		swayFreq := randFloat(1.2, 2.5)
		phase := randFloat(0, float32(2*math.Pi))

		balloons[i] = balloonItem{
			img:      img,
			w:        bw,
			h:        bh,
			baseX:    baseX,
			y:        initY,
			vy:       vy,
			swayAmp:  swayAmp,
			swayFreq: swayFreq,
			phase:    phase,
		}
		img.Move(fyne.NewPos(baseX, initY))
		canvasContainer.Add(img)
	}

	go func() {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()

		var elapsed float32
		lastTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				dt := float32(now.Sub(lastTime).Seconds())
				if dt > 0.1 {
					dt = 0.033
				}
				lastTime = now
				elapsed += dt

				fyne.Do(func() {
					select {
					case <-ctx.Done():
						return
					default:
					}

					curW, curH := getScreenBounds(root)
					for i := range balloons {
						b := &balloons[i]
						b.y -= b.vy * dt
						sway := b.swayAmp * float32(math.Sin(
							float64(elapsed*b.swayFreq+b.phase),
						))
						x := b.baseX + sway
						if b.y < -b.h {
							b.y = curH + randFloat(10, 80)
							b.baseX = randFloat(20, curW-b.w-20)
							b.phase = randFloat(0, float32(2*math.Pi))
							b.vy = randFloat(65, 110)
						}
						b.img.Move(fyne.NewPos(x, b.y))
						b.img.Refresh()
					}
				})
			}
		}
	}()
}

func startWeddingRingsAnimation(
	ctx context.Context,
	canvasContainer *fyne.Container,
	root fyne.CanvasObject,
) {
	_, h := getScreenBounds(root)

	ringsW := float32(150)
	ringsH := float32(150)
	rings := ringsItem{
		w:       ringsW,
		h:       ringsH,
		x:       -ringsW - 30,
		baseY:   h * 0.35,
		vx:      80,
		bobAmp:  30,
		bobFreq: 2.0,
	}

	sparkleCount := 7
	sparkles := make([]sparkleItem, sparkleCount)
	sparkleOffsets := [][2]float32{
		{-35, -20},
		{145, -15},
		{55, -45},
		{-20, 110},
		{130, 95},
		{60, 130},
		{-50, 45},
	}

	ringsImg := canvas.NewImageFromResource(loadIconResource("rings.svg"))
	ringsImg.FillMode = canvas.ImageFillContain
	ringsImg.Resize(fyne.NewSize(ringsW, ringsH))
	rings.img = ringsImg
	canvasContainer.Add(ringsImg)

	for i := 0; i < sparkleCount; i++ {
		sw := randFloat(22, 36)
		sImg := canvas.NewImageFromResource(loadIconResource("sparkle.svg"))
		sImg.FillMode = canvas.ImageFillContain
		sImg.Resize(fyne.NewSize(sw, sw))
		sparkles[i] = sparkleItem{
			img:     sImg,
			w:       sw,
			h:       sw,
			relX:    sparkleOffsets[i][0],
			relY:    sparkleOffsets[i][1],
			orbFreq: randFloat(1.5, 3.0),
			phase:   randFloat(0, float32(2*math.Pi)),
		}
		canvasContainer.Add(sImg)
	}

	go func() {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()

		var elapsed float32
		lastTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				dt := float32(now.Sub(lastTime).Seconds())
				if dt > 0.1 {
					dt = 0.033
				}
				lastTime = now
				elapsed += dt

				fyne.Do(func() {
					select {
					case <-ctx.Done():
						return
					default:
					}

					curW, curH := getScreenBounds(root)
					rings.x += rings.vx * dt
					rings.y = rings.baseY + rings.bobAmp*float32(math.Sin(
						float64(elapsed*rings.bobFreq),
					))
					if rings.x > curW+50 {
						rings.x = -rings.w - 50
						rings.baseY = randFloat(curH*0.25, curH*0.45)
					}
					if rings.img != nil {
						rings.img.Move(fyne.NewPos(rings.x, rings.y))
						rings.img.Refresh()
					}

					for i := range sparkles {
						sp := &sparkles[i]
						orbX := 12 * float32(math.Cos(
							float64(elapsed*sp.orbFreq+sp.phase),
						))
						orbY := 12 * float32(math.Sin(
							float64(elapsed*sp.orbFreq+sp.phase),
						))
						sx := rings.x + sp.relX + orbX
						sy := rings.y + sp.relY + orbY
						sp.img.Move(fyne.NewPos(sx, sy))
						sp.img.Refresh()
					}
				})
			}
		}
	}()
}

func startPartyAndConfettiAnimation(
	ctx context.Context,
	cType string,
	canvasContainer *fyne.Container,
	root fyne.CanvasObject,
) {
	w, h := getScreenBounds(root)

	confettiColors := []color.NRGBA{
		{R: 255, G: 77, B: 109, A: 255},  // Hot pink
		{R: 255, G: 209, B: 102, A: 255}, // Gold
		{R: 6, G: 182, B: 212, A: 255},   // Cyan
		{R: 16, G: 185, B: 129, A: 255},  // Emerald
		{R: 168, G: 85, B: 247, A: 255},  // Purple
		{R: 249, G: 115, B: 22, A: 255},  // Orange
	}

	confettiCount := 20
	particles := make([]confettiItem, confettiCount)

	for i := 0; i < confettiCount; i++ {
		pw := randFloat(8, 14)
		ph := randFloat(8, 16)
		cColor := confettiColors[i%len(confettiColors)]
		rect := canvas.NewRectangle(cColor)
		rect.Resize(fyne.NewSize(pw, ph))

		baseX := randFloat(20, w-pw-20)
		initY := randFloat(-50, h)
		vy := randFloat(70, 130)
		swayAmp := randFloat(15, 35)
		swayFreq := randFloat(1.5, 3.0)
		phase := randFloat(0, float32(2*math.Pi))

		particles[i] = confettiItem{
			rect:     rect,
			baseX:    baseX,
			y:        initY,
			vy:       vy,
			swayAmp:  swayAmp,
			swayFreq: swayFreq,
			phase:    phase,
		}
		rect.Move(fyne.NewPos(baseX, initY))
		canvasContainer.Add(rect)
	}

	iconFile := "party.svg"
	if strings.ToLower(cType) == "graduation" {
		iconFile = "graduation.svg"
	} else if strings.ToLower(cType) == "school" {
		iconFile = "school.svg"
	}
	leadIcon := canvas.NewImageFromResource(loadIconResource(iconFile))
	leadIcon.FillMode = canvas.ImageFillContain
	leadIcon.Resize(fyne.NewSize(110, 110))
	var leadX float32 = -130
	var leadY float32 = h * 0.3
	var leadVX float32 = 70
	canvasContainer.Add(leadIcon)

	go func() {
		ticker := time.NewTicker(33 * time.Millisecond)
		defer ticker.Stop()

		var elapsed float32
		lastTime := time.Now()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				dt := float32(now.Sub(lastTime).Seconds())
				if dt > 0.1 {
					dt = 0.033
				}
				lastTime = now
				elapsed += dt

				fyne.Do(func() {
					select {
					case <-ctx.Done():
						return
					default:
					}

					curW, curH := getScreenBounds(root)
					for i := range particles {
						p := &particles[i]
						p.y += p.vy * dt
						sway := p.swayAmp * float32(math.Sin(
							float64(elapsed*p.swayFreq+p.phase),
						))
						x := p.baseX + sway
						if p.y > curH+20 {
							p.y = -20
							p.baseX = randFloat(20, curW-30)
							p.phase = randFloat(0, float32(2*math.Pi))
						}
						p.rect.Move(fyne.NewPos(x, p.y))
						p.rect.Refresh()
					}

					if leadIcon != nil {
						leadX += leadVX * dt
						leadY = curH*0.32 + 25*float32(
							math.Sin(float64(elapsed*1.8)),
						)
						if leadX > curW+40 {
							leadX = -130
						}
						leadIcon.Move(fyne.NewPos(leadX, leadY))
						leadIcon.Refresh()
					}
				})
			}
		}
	}()
}

func startCelebrationAnimation(
	ctx context.Context,
	cType string,
	canvasContainer *fyne.Container,
	root fyne.CanvasObject,
) {
	switch strings.ToLower(strings.TrimSpace(cType)) {
	case "birthday":
		startBalloonsAnimation(ctx, canvasContainer, root)
	case "anniversary":
		startWeddingRingsAnimation(ctx, canvasContainer, root)
	default:
		startPartyAndConfettiAnimation(ctx, cType, canvasContainer, root)
	}
}

// CreatePhotoOverlayView builds the overlay container that pops up over the
// slideshow when celebrations trigger.
func CreatePhotoOverlayView() fyne.CanvasObject {
	bannerText := canvas.NewText(
		"🎉 CELEBRATION 🎉",
		color.NRGBA{R: 255, G: 215, B: 0, A: 255},
	)
	bannerText.TextSize = 22
	bannerText.TextStyle.Bold = true
	bannerText.Alignment = fyne.TextAlignCenter

	messageText := canvas.NewText("", color.White)
	messageText.TextSize = 26
	messageText.TextStyle.Bold = true
	messageText.Alignment = fyne.TextAlignCenter

	titleText := canvas.NewText(
		"", color.NRGBA{R: 220, G: 220, B: 240, A: 255},
	)
	titleText.TextSize = 16
	titleText.Alignment = fyne.TextAlignCenter

	iconImg := canvas.NewImageFromResource(loadIconResource("balloons.svg"))
	iconImg.FillMode = canvas.ImageFillContain
	iconImg.SetMinSize(fyne.NewSize(54, 54))

	dismissBtn := widget.NewButtonWithIcon(
		"Dismiss", theme.CancelIcon(), nil,
	)
	dismissBtn.Importance = widget.LowImportance

	bg := canvas.NewRectangle(color.NRGBA{R: 15, G: 20, B: 35, A: 240})
	bg.CornerRadius = 16
	bg.StrokeColor = color.NRGBA{R: 255, G: 215, B: 0, A: 140}
	bg.StrokeWidth = 2

	centerContent := container.NewVBox(
		bannerText,
		messageText,
		titleText,
	)

	cardBorder := container.NewBorder(
		nil, nil,
		container.NewPadded(iconImg),
		container.NewCenter(container.NewPadded(dismissBtn)),
		container.NewCenter(centerContent),
	)

	card := container.New(
		layout.NewMaxLayout(),
		bg,
		container.NewPadded(cardBorder),
	)

	// Anchored at the bottom of the screen with padding
	messageOverlay := container.NewBorder(
		nil,
		container.NewPadded(container.NewCenter(card)),
		nil,
		nil,
	)

	animationCanvas := container.NewWithoutLayout()

	rootContainer := container.NewMax(
		animationCanvas,
		messageOverlay,
	)
	rootContainer.Hide()

	var dismissTimer *time.Timer
	var timerMu sync.Mutex
	var animCancel context.CancelFunc
	var animMu sync.Mutex

	stopAnimation := func() {
		animMu.Lock()
		if animCancel != nil {
			animCancel()
			animCancel = nil
		}
		animMu.Unlock()
	}

	hideOverlay := func() {
		timerMu.Lock()
		if dismissTimer != nil {
			dismissTimer.Stop()
			dismissTimer = nil
		}
		timerMu.Unlock()

		stopAnimation()

		fyne.Do(func() {
			animationCanvas.RemoveAll()
			rootContainer.Hide()
			rootContainer.Refresh()
		})
	}

	dismissBtn.OnTapped = hideOverlay

	RegisterDisplayHandler(func(c database.Celebration) {
		fyne.Do(func() {
			timerMu.Lock()
			if dismissTimer != nil {
				dismissTimer.Stop()
			}
			dismissTimer = time.AfterFunc(DisplayDuration, hideOverlay)
			timerMu.Unlock()

			stopAnimation()
			animationCanvas.RemoveAll()

			bannerText.Text = GetBannerText(c)
			bannerText.Refresh()

			message := c.Message
			if message == "" {
				message = c.Title
			}
			messageText.Text = message
			messageText.Refresh()

			if c.Title != "" && c.Title != message {
				titleText.Text = c.Title
				titleText.Show()
			} else {
				titleText.Hide()
			}
			titleText.Refresh()

			iconImg.Resource = GetIconForType(c.Type)
			iconImg.Refresh()

			rootContainer.Show()
			rootContainer.Refresh()

			animCtx, cancel := context.WithCancel(context.Background())
			animMu.Lock()
			animCancel = cancel
			animMu.Unlock()

			startCelebrationAnimation(animCtx, c.Type, animationCanvas, rootContainer)
		})
	})

	return rootContainer
}
