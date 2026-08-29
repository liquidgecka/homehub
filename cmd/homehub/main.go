//   Copyright 2026 - Brady Catherman
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package main

import (
	"context"
	"flag"
	"image/color"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	gcalendar "google.golang.org/api/calendar/v3"

	"github.com/liquidgecka/homehub/activity"
	"github.com/liquidgecka/homehub/background"
	"github.com/liquidgecka/homehub/calendar"
	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/database"
	"github.com/liquidgecka/homehub/dialogs"
	"github.com/liquidgecka/homehub/dpms"
	"github.com/liquidgecka/homehub/home"
	"github.com/liquidgecka/homehub/ledger"
	"github.com/liquidgecka/homehub/logging"
	"github.com/liquidgecka/homehub/photomanager"
	"github.com/liquidgecka/homehub/reminders"
	"github.com/liquidgecka/homehub/security"
	"github.com/liquidgecka/homehub/shopping"
	"github.com/liquidgecka/homehub/ui"
	"github.com/liquidgecka/homehub/weather"
	"github.com/liquidgecka/homehub/web"
)

// Global variable to hold the content area, allowing view switching.
var currentContent *fyne.Container
var selectedButton *navButton
var homeView fyne.CanvasObject
var homeButton *navButton
var softGrey color.NRGBA
var calendarService *gcalendar.Service
var idleTimer *time.Timer
var idleWindow fyne.Window // To store a reference to the main window
var sidebar *fyne.Container
var cancelCalendarView context.CancelFunc
var cancelDPMS context.CancelFunc
var cancelTasksSync context.CancelFunc

// Global SlideshowManager instance and its stop function
var slideshowManager *home.SlideshowManager

// resetIdleTimer resets the idle timer.
func resetIdleTimer() {
	if idleTimer != nil {
		idleTimer.Stop()
	}
	timeoutMinutes := config.GetConfig().App.IdleTimeoutMinutes
	idleDuration := time.Duration(timeoutMinutes) * time.Minute
	idleTimer = time.AfterFunc(idleDuration, onIdle)
	log.Println("Idle timer reset.")
}

// onIdle is called when the idle timer expires.
func onIdle() {
	log.Println("Idle timeout reached. Returning to Home view.")
	dialogs.CloseAll()
	fyne.Do(func() {
		if selectedButton != nil && selectedButton.updateContent != nil {
			selectedButton.updateContent()
			selectedButton.updateContent = nil
		}
		if currentContent != nil {
			currentContent.Objects = []fyne.CanvasObject{homeView}
			currentContent.Refresh()
		}
		if selectedButton != nil {
			selectedButton.Importance = widget.LowImportance
			selectedButton.Refresh()
		}
		if homeButton != nil {
			homeButton.Importance = widget.HighImportance
			selectedButton = homeButton
			homeButton.Refresh()
		} else {
			log.Println("Global homeButton is nil, cannot set as selected.")
		}
	})
}

// createMainLayout constructs the main UI layout of the application.
func createMainLayout() fyne.CanvasObject {
	return container.NewBorder(
		nil,            // Top
		nil,            // Bottom
		sidebar,        // Left (fixed width controlled by MinSize of sidebar)
		nil,            // Right
		currentContent, // Center (fills remaining space)
	)
}

type navButton struct {
	widget.BaseWidget
	iconResource  fyne.Resource
	onTapped      func()
	background    *canvas.Rectangle
	scaledIcon    *widget.Icon
	Importance    widget.ButtonImportance // Add Importance field
	updateContent func()
}

func (n *navButton) CreateRenderer() fyne.WidgetRenderer {
	n.ExtendBaseWidget(n)
	n.background = canvas.NewRectangle(softGrey)
	n.background.CornerRadius = 10
	n.scaledIcon = widget.NewIcon(n.iconResource)
	return &navButtonRenderer{button: n}
}

func (n *navButton) MinSize() fyne.Size {
	n.ExtendBaseWidget(n)
	iconSize := theme.IconInlineSize() * 2 // Reduce button size to 2x
	return fyne.NewSize(iconSize*1.2, iconSize*1.2)
}

func (n *navButton) Tapped(event *fyne.PointEvent) {
	if activity.ResetTimer != nil {
		activity.ResetTimer()
	}
	n.onTapped()
	n.Refresh()
}

// TappedSecondary handles secondary taps (no-op for nav buttons).
func (n *navButton) TappedSecondary(event *fyne.PointEvent) {
	// No secondary action for nav buttons
}

type navButtonRenderer struct {
	button  *navButton
	objects []fyne.CanvasObject
}

func (r *navButtonRenderer) MinSize() fyne.Size {
	return r.button.MinSize()
}

func (r *navButtonRenderer) Layout(size fyne.Size) {
	r.button.background.Resize(size)
	r.button.background.Move(fyne.NewPos(0, 0))

	xOffset := float32(0)
	if r.button.iconResource.Name() == "dollar.svg" {
		xOffset = -40
	} else if r.button.iconResource.Name() == "cloud.svg" {
		xOffset = -40
	} else if r.button.iconResource.Name() == theme.CalendarIcon().Name() {
		xOffset = 0
	}

	// Center the scaled icon within the button
	iconDimension := theme.IconInlineSize() * 2
	desiredIconSize := fyne.NewSize(iconDimension, iconDimension)
	r.button.scaledIcon.Resize(desiredIconSize)
	r.button.scaledIcon.Move(
		fyne.NewPos(
			(size.Width-desiredIconSize.Width)/2+xOffset,
			(size.Height-desiredIconSize.Height)/2,
		),
	)
}

func (r *navButtonRenderer) Refresh() {
	if r.button.Importance == widget.HighImportance {
		r.button.background.FillColor = theme.PrimaryColor()
	} else {
		r.button.background.FillColor = softGrey
	}
	r.button.background.Refresh()
	r.button.scaledIcon.Refresh()
}

func (r *navButtonRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.objects = []fyne.CanvasObject{
			r.button.background, r.button.scaledIcon,
		}
	}
	return r.objects
}

func (r *navButtonRenderer) Destroy() {
	// Nothing to destroy explicitly, Fyne handles canvas objects
}

// createNavButton creates a button for navigation.
func createNavButton(
	icon fyne.Resource,
	content func() (fyne.CanvasObject, func()),
	contentContainer *fyne.Container,
	win fyne.Window,
) *navButton {
	var button *navButton
	button = &navButton{
		iconResource: icon,
	}
	button.onTapped = func() {
		if selectedButton != nil {
			selectedButton.Importance = widget.LowImportance
			selectedButton.Refresh()
			if selectedButton.updateContent != nil {
				selectedButton.updateContent()
			}
		}
		button.Importance = widget.HighImportance
		contentContainer.RemoveAll()
		view, updateFn := content()
		contentContainer.Add(view)
		button.updateContent = updateFn
		contentContainer.Refresh()
		selectedButton = button
	}
	button.ExtendBaseWidget(button) // Initialize BaseWidget fields
	return button
}

var (
	// Reference to ensure weather package is imported
	_ *weather.OpenWeather
)

func main() {
	validateConfigFlag := flag.Bool(
		"validate-config", false, "Validate the config.toml file and exit",
	)
	configPath := flag.String(
		"config", config.GetDefaultConfigPath(), "Path to config.toml file",
	)
	flag.Parse()
	if *validateConfigFlag {
		runConfigValidation(*configPath)
		os.Exit(0)
	}

	log.Println("Starting application...")
	log.Println("Attempting to load configuration...")
	if err := config.LoadConfig(*configPath); err != nil {
		log.Fatalf(
			"Error loading configuration from %s: %v", *configPath, err,
		)
	}
	log.Println("Configuration loaded successfully.")

	// Initialize rotating file logger
	logWriter, err := logging.InitLogger(config.GetConfig())
	if err != nil {
		log.Printf("Warning: Failed to initialize file logger: %v", err)
	} else {
		defer logWriter.Close()
	}

	go func() {
		log.Println("Starting pprof server on :6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	if err := database.OpenFileDB(); err != nil {
		log.Fatalf("Failed to open database file: %v", err)
	}
	if err := database.InitDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	activity.ResetTimer = resetIdleTimer
	log.Println("Initializing Google services...")
	if _, err := calendar.InitGoogleCalendarClient(); err != nil {
		log.Fatalf("Failed to initialize Google Calendar client: %v", err)
	}
	log.Println("Google services initialized.")
	log.Println("Updating shopping store metadata...")
	for i, store := range config.GetConfig().Shopping.Store {
		if !store.Disabled {
			storeID := i + 1
			if err := database.AddOrUpdateShoppingStoreMetadata(
				storeID, time.Now(),
			); err != nil {
				log.Printf(
					"Failed to update metadata for store %d: %v",
					storeID, err,
				)
			}
		}
	}
	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	log.Println("Starting background tasks...")
	bgManager := background.NewManager()
	bgManager.Init()
	bgManager.Start()
	defer bgManager.Stop()
	cancelTasksSync = shopping.StartGoogleTasksSync(appCtx)
	defer cancelTasksSync()
	go security.StartMqttListener(appCtx)
	log.Printf("DPMS: Initializing with config: %+v", config.GetConfig().DPMS)
	cancelDPMS = dpms.StartScheduler(appCtx, &config.GetConfig().DPMS)
	defer cancelDPMS()
	log.Println("Background tasks started.")
	web.Start(&config.GetConfig().App)
	softGrey = color.NRGBA{R: 0x30, G: 0x30, B: 0x30, A: 0xFF}

	// Initialize the global slideshow manager once
	slideshowCfg := home.SlideshowConfig{
		Directory: config.GetConfig().LocalPhotos.Directory,
		RotationIntervalSeconds: config.GetConfig().
			LocalPhotos.RotationIntervalSeconds,
	}
	slideshowManager = home.NewSlideshowManager(appCtx, slideshowCfg)
	slideshowManager.Start() // Start its goroutine

	a := app.New()

	a.Settings().SetTheme(&myTheme{Theme: a.Settings().Theme()})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down due to signal...")
		a.Quit()
	}()
	w := a.NewWindow("HomeHub")
	currentContent = container.New(layout.NewMaxLayout())

	// Create the home view and start its listener once.
	var img *canvas.Image
	var label *widget.Label
	var heartButton, hideButton *ui.TappableIcon
	homeView, img, label, heartButton, hideButton = home.CreateView()
	slideshowStopFunc := home.StartSlideshowAndPhotoListener(
		img, label, heartButton, hideButton, slideshowManager,
	)
	defer slideshowStopFunc()
	slideshowManager.ResendState()

	currentContent.Objects = []fyne.CanvasObject{homeView}
	currentContent.Refresh()

	homeButton = createNavButton(
		theme.HomeIcon(),
		func() (fyne.CanvasObject, func()) {
			slideshowManager.ResendState()
			return homeView, nil
		},
		currentContent,
		w,
	)
	managementButton := createNavButton(
		theme.ListIcon(),
		func() (fyne.CanvasObject, func()) {
			return photomanager.CreateManagementView(w), nil
		},
		currentContent,
		w,
	)
	var cancelCalendarView context.CancelFunc
	calendarButton := createNavButton(
		theme.CalendarIcon(),
		func() (fyne.CanvasObject, func()) {
			if cancelCalendarView != nil {
				cancelCalendarView()
			}
			view, cancel, _ := calendar.CreateCalendarView(
				calendar.GetCalendarService(),
			)
			cancelCalendarView = cancel
			return view, cancel
		},
		currentContent,
		w,
	)
	shoppingStaticIcon := fyne.NewStaticResource(
		"shopping-cart.svg",
		ui.MustLoadFile(ui.GetIconPath("shopping-cart.svg")),
	)
	shoppingIcon := theme.NewThemedResource(shoppingStaticIcon)
	shoppingButton := createNavButton(
		shoppingIcon,
		func() (fyne.CanvasObject, func()) {
			return createShoppingContent(w)
		},
		currentContent,
		w,
	)
	bellStaticIcon := fyne.NewStaticResource(
		"bell.svg",
		ui.MustLoadFile(ui.GetIconPath("bell.svg")),
	)
	bellIcon := theme.NewThemedResource(bellStaticIcon)
	remindersButton := createNavButton(
		bellIcon,
		func() (fyne.CanvasObject, func()) {
			return reminders.CreateRemindersView(w, currentContent)
		},
		currentContent,
		w,
	)
	cloudStaticIcon := fyne.NewStaticResource(
		"cloud.svg",
		ui.MustLoadFile(ui.GetIconPath("cloud.svg")),
	)
	cloudIcon := theme.NewThemedResource(cloudStaticIcon)
	weatherButton := createNavButton(
		cloudIcon,
		func() (fyne.CanvasObject, func()) {
			return weather.CreateView(config.GetConfig()), nil
		},
		currentContent,
		w,
	)
	var makeFinanceView func() (fyne.CanvasObject, func())
	makeFinanceView = func() (fyne.CanvasObject, func()) {
		return createFinanceContent(w, currentContent), nil
	}
	financeStaticIcon := fyne.NewStaticResource(
		"dollar.svg",
		ui.MustLoadFile(ui.GetIconPath("dollar.svg")),
	)
	financeButton := createNavButton(
		theme.NewThemedResource(financeStaticIcon),
		makeFinanceView,
		currentContent,
		w,
	)
	var securityButton *navButton
	if len(config.GetConfig().Security.Camera) > 0 {
		securityStaticIcon := fyne.NewStaticResource(
			"camera.svg",
			ui.MustLoadFile(ui.GetIconPath("camera.svg")),
		)
		securityIcon := theme.NewThemedResource(securityStaticIcon)
		securityButton = createNavButton(
			securityIcon,
			func() (fyne.CanvasObject, func()) {
				view := security.New(w)
				return view.GetContent(), view.Stop
			},
			currentContent,
			w,
		)
	}
	navButtons := []fyne.CanvasObject{
		homeButton,
		managementButton,
		calendarButton,
		shoppingButton,
		remindersButton,
		weatherButton,
		financeButton,
	}
	if securityButton != nil {
		navButtons = append(navButtons, securityButton)
	}
	sidebar = container.NewPadded(
		container.NewGridWithColumns(1, navButtons...),
	)
	homeButton.onTapped()
	selectedButton = homeButton
	go func() {
		for {
			select {
			case <-appCtx.Done():
				return
			case event := <-security.CameraEventChan:
				fyne.Do(func() {
					if event.Type == "start" && securityButton != nil {
						if selectedButton != securityButton {
							securityButton.onTapped()
						}
					} else if event.Type == "end" && homeButton != nil {
						if selectedButton != homeButton {
							homeButton.onTapped()
						}
					}
				})
			}
		}
	}()
	w.SetContent(createMainLayout())
	idleWindow = w
	resetIdleTimer()
	w.SetFullScreen(true)
	w.Show()

	// Defer the stop functions to be called when main exits.
	defer slideshowManager.Stop() // Ensure the manager itself is stopped
	a.Run()
}

func runConfigValidation(configPath string) {
	log.Println("Running config validation...")
	if err := config.LoadConfig(configPath); err != nil {
		log.Fatalf(
			"Error loading config for validation from %s: %v", configPath, err,
		)
	}
	if err := config.ValidateConfig(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}
	log.Println("Configuration validated successfully.")
}

// createShoppingContent creates the shopping view and its refresh callback.
func createShoppingContent(win fyne.Window) (fyne.CanvasObject, func()) {
	return shopping.CreateShoppingView(win, currentContent)
}

// createFinanceContent creates the finance view and its refresh callback.
func createFinanceContent(
	win fyne.Window, contentContainer *fyne.Container,
) fyne.CanvasObject {
	var financeRefreshCallback func()
	financeRefreshCallback = func() {
		contentContainer.Objects = []fyne.CanvasObject{
			createFinanceContent(win, contentContainer),
		}
		contentContainer.Refresh()
	}
	return ledger.CreateFinanceView(win, financeRefreshCallback)
}
