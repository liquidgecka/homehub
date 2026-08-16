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
	"fmt"
	"image/color"
	"log"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	gcalendar "google.golang.org/api/calendar/v3"

	"github.com/liquidgecka/homehub/config"
	"github.com/liquidgecka/homehub/dialogs"
	"github.com/liquidgecka/homehub/ui"
)

type hourLines struct {
	widget.BaseWidget
	hourHeight float32
}

func newHourLines(hourHeight float32) *hourLines {
	h := &hourLines{hourHeight: hourHeight}
	h.ExtendBaseWidget(h)
	return h
}

func (h *hourLines) CreateRenderer() fyne.WidgetRenderer {
	lines := []fyne.CanvasObject{}
	for i := 0; i < 24; i++ {
		line := canvas.NewLine(theme.SeparatorColor())
		lines = append(lines, line)
	}
	currentTimeLine := canvas.NewLine(color.NRGBA{R: 255, G: 0, B: 0, A: 150})
	currentTimeLine.StrokeWidth = 2
	lines = append(lines, currentTimeLine)

	r := &hourLinesRenderer{
		lines:      lines,
		hourHeight: h.hourHeight,
	}
	return r
}

type hourLinesRenderer struct {
	lines      []fyne.CanvasObject
	hourHeight float32
}

func (r *hourLinesRenderer) Layout(size fyne.Size) {
	for i, obj := range r.lines {
		line := obj.(*canvas.Line)
		if i < 24 { // Hour lines
			y := (float32(i) * r.hourHeight)
			line.Position1 = fyne.NewPos(0, y)
			line.Position2 = fyne.NewPos(size.Width, y)
		} else { // Current time line
			now := time.Now().In(time.Local)
			yPos := (float32(now.Hour()) + float32(now.Minute())/60.0) * r.hourHeight
			line.Position1 = fyne.NewPos(0, yPos)
			line.Position2 = fyne.NewPos(size.Width, yPos)
		}
	}
}

func (r *hourLinesRenderer) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}

func (r *hourLinesRenderer) Refresh() {
	for _, obj := range r.lines {
		obj.Refresh()
	}
}

func (r *hourLinesRenderer) Objects() []fyne.CanvasObject {
	return r.lines
}

func (r *hourLinesRenderer) Destroy() {}

var currentCalendarDate time.Time
var calendarViewMode = "month" // "month" or "week"

// --- Custom Widgets and Layouts ---

type weekHeaderLayout struct {
	spacerWidth float32
}

func (w *weekHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	// objects[0] is headerSpacer
	// objects[1] is dayNamesHeader
	objects[0].Resize(fyne.NewSize(w.spacerWidth, size.Height))
	objects[0].Move(fyne.NewPos(0, 0))

	remainingWidth := size.Width - w.spacerWidth
	objects[1].Resize(fyne.NewSize(remainingWidth, size.Height))
	objects[1].Move(fyne.NewPos(w.spacerWidth, 0))
}

func (w *weekHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	spacerMin := objects[0].MinSize()
	headerMin := objects[1].MinSize()
	return fyne.NewSize(spacerMin.Width+headerMin.Width, fyne.Max(spacerMin.Height, headerMin.Height))
}

type unexpandingGrid struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func (u *unexpandingGrid) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(u.content)
}
func (u *unexpandingGrid) MinSize() fyne.Size {
	return fyne.NewSize(0, 0)
}
func newUnexpandingGrid(content fyne.CanvasObject) *unexpandingGrid {
	grid := &unexpandingGrid{content: content}
	grid.ExtendBaseWidget(grid)
	return grid
}

type eventBox struct {
	widget.BaseWidget
	event  *gcalendar.Event
	label  *widget.Label
	bg     *canvas.Rectangle
	border *canvas.Rectangle
}

func (e *eventBox) CreateRenderer() fyne.WidgetRenderer {
	// Wrap label in a VBox to ensure top alignment
	return widget.NewSimpleRenderer(container.NewMax(e.bg, e.border, container.NewVBox(e.label)))
}

func newEventBox(event *gcalendar.Event, cfg *config.Config) *eventBox {

	if event == nil {

		return &eventBox{

			label: widget.NewLabel("Error: Event is nil"),

			bg: canvas.NewRectangle(color.Gray{}),

			border: canvas.NewRectangle(color.Black),
		}

	}

	// Defensive checks for cfg fields, although GetConfig should prevent nil

	if cfg == nil {

		log.Println("ERROR: newEventBox received nil config. Using fallback.")

		return &eventBox{label: widget.NewLabel("Error: Config is nil"), bg: canvas.NewRectangle(color.Gray{}), border: canvas.NewRectangle(color.Black)}

	}

	eventTimeFormatted := "N/A"

	if event.Start != nil { // Check if Start is present to avoid panic if event.Start is nil

		eventTimeFormatted = getEventStartTime(event).Format(cfg.Google.Calendar.TimeFormat)

	}

	eventSummarySafe := "No Description"

	if event.Summary != "" { // event.Summary is a string, can be empty but not nil

		eventSummarySafe = event.Summary

	}

	formattedText := fmt.Sprintf("%s\n%s", eventSummarySafe, eventTimeFormatted)

	newBg := canvas.NewRectangle(theme.PrimaryColor())

	if newBg == nil {

		log.Println("ERROR: newEventBox - canvas.NewRectangle(theme.PrimaryColor()) returned nil. Using fallback.")

		newBg = canvas.NewRectangle(color.Gray{}) // Fallback

	}

	newBorder := canvas.NewRectangle(color.Transparent)

	if newBorder == nil {

		log.Println("ERROR: newEventBox - canvas.NewRectangle(color.Transparent) returned nil. Using fallback.")

		newBorder = canvas.NewRectangle(color.Black) // Fallback

	}

	box := &eventBox{

		event: event,

		label: widget.NewLabel(formattedText),

		bg: newBg,

		border: newBorder,
	}

	box.bg.CornerRadius = 3
	box.label.Wrapping = fyne.TextWrapWord
	box.label.Alignment = fyne.TextAlignLeading
	box.border.StrokeColor = color.White // Make border visible
	box.border.StrokeWidth = 1
	box.ExtendBaseWidget(box)
	return box
}

type dayLayout struct {
	cfg        *config.Config
	hourHeight float32
}

func (d *dayLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var events []*eventBox
	for _, obj := range objects {
		if event, ok := obj.(*eventBox); ok {
			events = append(events, event)
		}
	}
	if len(events) == 0 {
		return
	}

	sort.Slice(events, func(i, j int) bool {
		return getEventStartTime(events[i].event).Before(getEventStartTime(events[j].event))
	})

	adj := make([][]int, len(events))
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if eventsOverlap(events[i], events[j]) {
				adj[i] = append(adj[i], j)
				adj[j] = append(adj[j], i)
			}
		}
	}

	visited := make([]bool, len(events))
	for i := 0; i < len(events); i++ {
		if !visited[i] {
			groupIndices := []int{}
			q := []int{i}
			visited[i] = true
			head := 0
			for head < len(q) {
				u := q[head]
				head++
				groupIndices = append(groupIndices, u)
				for _, v := range adj[u] {
					if !visited[v] {
						visited[v] = true
						q = append(q, v)
					}
				}
			}
			d.layoutGroup(events, groupIndices, size)
		}
	}
}

func (d *dayLayout) layoutGroup(allEvents []*eventBox, groupIndices []int, size fyne.Size) {
	groupEvents := []*eventBox{}
	for _, index := range groupIndices {
		groupEvents = append(groupEvents, allEvents[index])
	}
	if len(groupEvents) == 0 {
		return
	}

	sort.Slice(groupEvents, func(i, j int) bool {
		return getEventStartTime(groupEvents[i].event).Before(getEventStartTime(groupEvents[j].event))
	})

	columns := [][]*eventBox{}
	for _, event := range groupEvents {
		placed := false
		for i, col := range columns {
			lastEventInCol := col[len(col)-1]
			if !eventsOverlap(lastEventInCol, event) {
				columns[i] = append(columns[i], event)
				placed = true
				break
			}
		}
		if !placed {
			columns = append(columns, []*eventBox{event})
		}
	}

	numCols := len(columns)
	horizontalPadding := float32(2)
	availableWidth := size.Width - (2 * horizontalPadding)
	if availableWidth < 0 {
		availableWidth = 0
	}
	colWidth := availableWidth / float32(numCols)

	for colIndex, col := range columns {
		for _, event := range col {
			totalMinutes := float32(24 * 60)
			startTime := getEventStartTime(event.event)
			endTime := getEventEndTime(event.event)
			startOfDay := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
			startMinute := float32(startTime.Sub(startOfDay).Minutes())
			endMinute := float32(endTime.Sub(startOfDay).Minutes())
			if startMinute < 0 {
				startMinute = 0
			}
			if endMinute > totalMinutes {
				endMinute = totalMinutes
			}
			yPos := (startMinute / totalMinutes) * size.Height
			height := ((endMinute - startMinute) / totalMinutes) * size.Height
			if height < 0 {
				height = 0
			}
			if height < theme.Padding()*2 {
				height = theme.Padding() * 2
			}

			eventX := horizontalPadding + float32(colIndex)*colWidth
			eventWidth := colWidth - horizontalPadding
			if eventWidth < 0 {
				eventWidth = 0
			}

			event.Move(fyne.NewPos(eventX, yPos))
			event.Resize(fyne.NewSize(eventWidth, height))
		}
	}
}

func (d *dayLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(10, 24*d.hourHeight) // Min height for a full day
}

// --- Helper Functions ---

func getEventStartTime(event *gcalendar.Event) time.Time {
	if event.Start != nil {
		if event.Start.DateTime != "" {
			t, _ := time.Parse(time.RFC3339, event.Start.DateTime)
			return t
		} else if event.Start.Date != "" {
			t, _ := time.Parse("2006-01-02", event.Start.Date)
			return t
		}
	}
	return time.Time{}
}

func getEventEndTime(event *gcalendar.Event) time.Time {
	if event.End != nil {
		if event.End.DateTime != "" {
			t, _ := time.Parse(time.RFC3339, event.End.DateTime)
			return t
		} else if event.End.Date != "" {
			t, _ := time.Parse("2006-01-02", event.End.Date)
			return t
		}
	}
	return getEventStartTime(event).Add(1 * time.Hour)
}

func getEventDuration(event *gcalendar.Event) time.Duration {
	startTime := getEventStartTime(event)
	endTime := getEventEndTime(event)
	if endTime.Before(startTime) {
		return 0 // Handle malformed events
	}
	return endTime.Sub(startTime)
}

func eventsOverlap(e1, e2 *eventBox) bool {
	start1 := getEventStartTime(e1.event)
	end1 := getEventEndTime(e1.event)
	start2 := getEventStartTime(e2.event)
	end2 := getEventEndTime(e2.event)

	// Events overlap if they are not completely disjoint.
	return start1.Before(end2) && start2.Before(end1)
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func showEventDetailsDialog(parent fyne.Window, event *gcalendar.Event) {
	startTime := getEventStartTime(event).Format("Mon, Jan 2, 2006 at 3:04 PM")
	endTime := getEventEndTime(event).Format("Mon, Jan 2, 2006 at 3:04 PM")
	content := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Event", Widget: widget.NewLabel(event.Summary)},
			{Text: "Starts", Widget: widget.NewLabel(startTime)},
			{Text: "Ends", Widget: widget.NewLabel(endTime)},
			{Text: "Description", Widget: widget.NewLabel(event.Description)},
		},
	}
	dialogs.ShowCustomConfirm("Event Details", "Close", "", content, func(b bool) {}, parent)
}

// --- View Generation ---

func generateWeekGrid(ctx context.Context, week time.Time, calService *gcalendar.Service, config *config.Config) fyne.CanvasObject {
	events := CachedEvents
	if events == nil {
		return container.NewCenter(widget.NewLabel("Loading events..."))
	}

	hourHeight := float32(100)    // Height for each hour slot
	timeLabelWidth := float32(70) // Fixed width for "3 PM" label

	// --- Top Header: Month/Year Label ---
	monthYearLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("%s %d", week.Month(), week.Year()),
		fyne.TextAlignCenter,
		fyne.TextStyle{Bold: true},
	)

	// --- Top Header: Day Names (aligned with day columns, horizontally scrollable) ---
	dayNamesHeaderContent := container.New(layout.NewGridLayout(7))
	startOfWeek := week.AddDate(0, 0, -int(week.Weekday())) // Adjust to Sunday
	for i := 0; i < 7; i++ {
		day := startOfWeek.AddDate(0, 0, i)
		dayNamesHeaderContent.Add(widget.NewLabelWithStyle(
			fmt.Sprintf("%s %d", day.Weekday().String()[:3], day.Day()),
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		))
	}
	// This will be wrapped in a horizontal scroller later

	// --- Time Column (Left, fixed width, vertically scrollable with main content) ---
	var hourLabels []fyne.CanvasObject
	for i := 0; i < 24; i++ {
		t := time.Date(0, 0, 0, i, 0, 0, 0, time.Local)
		hourRect := canvas.NewRectangle(color.Transparent)
		hourRect.SetMinSize(fyne.NewSize(timeLabelWidth, hourHeight))
		hourLabels = append(hourLabels, container.NewMax(hourRect, container.NewCenter(widget.NewLabel(t.Format("3 PM")))))
	}
	timeLabelsContent := container.New(layout.NewVBoxLayout(), hourLabels...)

	// --- Main Grid (7 Day Columns with Events and Hour Lines, horizontally and vertically scrollable) ---
	dayColumns := []fyne.CanvasObject{}
	for i := 0; i < 7; i++ {
		day := startOfWeek.AddDate(0, 0, i) // Use startOfWeek for consistency
		var dayEvents []*gcalendar.Event
		for _, event := range events {
			if event.Start.DateTime == "" {
				continue
			}
			eventStart := getEventStartTime(event)
			// Filter events for this specific day
			if eventStart.Year() == day.Year() && eventStart.YearDay() == day.YearDay() {
				dayEvents = append(dayEvents, event)
			}
		}

		eventObjects := []fyne.CanvasObject{}
		for _, e := range dayEvents {
			eventObjects = append(eventObjects, newEventBox(e, config))
		}

		dayContainer := container.New(&dayLayout{cfg: config, hourHeight: hourHeight}, eventObjects...)
		dayBorder := canvas.NewRectangle(color.Transparent)
		dayBorder.StrokeColor = theme.SeparatorColor()
		dayBorder.StrokeWidth = 1
		dayColumns = append(dayColumns, container.NewMax(dayBorder, dayContainer))
	}

	gridOfDays := container.New(layout.NewGridLayout(7), dayColumns...)
	// Ensure gridOfDays has a minimum width so it doesn't collapse horizontally
	// Assuming a default column width, e.g., 200 pixels

	lines := newHourLines(hourHeight)
	gridWithLines := container.NewMax(lines, gridOfDays) // Hour lines overlaid on the grid of days

	// --- Assemble Main Content Area (Fixed Time Labels + Scrollable Day Content) ---
	// This will be vertically scrollable as a whole, and day content horizontally scrollable.
	mainContentBody := container.NewBorder(
		nil, nil, // top, bottom
		timeLabelsContent, // left
		nil,               // right
		gridWithLines,     // center
	)
	// mainContentBody.Layout.(*layout.HBoxLayout).SetHorizontalFit(true) // Allow grid to expand horizontally // REMOVED THIS LINE

	// --- Create a scroll container for the main content body (vertical scroll) ---
	mainContentScroller := container.NewScroll(mainContentBody)
	mainContentScroller.Direction = container.ScrollVerticalOnly // Only vertical scroll for this main area

	// Scroll to current time
	now := time.Now()
	yPos := (float32(now.Hour()) + float32(now.Minute())/60.0) * hourHeight
	// We want to center the view around the current time. To do this, we need the scroller's height.
	// Since the scroller's height is not known at this point, we can't perfectly center it.
	// A good approximation is to scroll to a few hours before the current time.
	yPos -= 3 * hourHeight // Scroll up by 3 hours
	if yPos < 0 {
		yPos = 0
	}
	mainContentScroller.Offset.Y = yPos

	// --- Top Section of the Week View ---
	// This includes the empty space for the time labels on the left, and the horizontally scrollable day names
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(timeLabelWidth, dayNamesHeaderContent.MinSize().Height))
	topDayHeader := container.NewBorder(nil, nil, spacer, nil, dayNamesHeaderContent)

	// --- Final Assembly ---
	// The overall layout: Month/Year fixed, then day headers, then main scrollable content
	topPanel := container.NewVBox(
		container.NewCenter(monthYearLabel), // Fixed month/year
		topDayHeader,                        // Horizontally scrollable day names
	)
	weekViewLayout := container.NewBorder(
		topPanel,            // Top
		nil,                 // Bottom
		nil,                 // Left
		nil,                 // Right
		mainContentScroller, // Center
	)

	// This goroutine refreshes the current time line.
	go func(lines *hourLines) {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				lines.Refresh()
			}
		}
	}(lines)

	return weekViewLayout
}

func generateCalendarGrid(month time.Time, calService *gcalendar.Service, config *config.Config) fyne.CanvasObject { // Removed events parameter
	// Access events directly from the global cache
	events := CachedEvents
	if events == nil { // Still check if the global cache is empty
		return container.NewCenter(widget.NewLabel("Loading events..."))
	}
	currentYear, currentMonth, _ := month.Date()

	eventsByDay := make(map[int][]*gcalendar.Event)
	for _, event := range events {
		eventTime := getEventStartTime(event)
		if eventTime.Year() == currentYear && eventTime.Month() == currentMonth {
			eventsByDay[eventTime.Day()] = append(eventsByDay[eventTime.Day()], event)
		}
	}

	for day := range eventsByDay {
		sort.Slice(eventsByDay[day], func(i, j int) bool {
			return getEventStartTime(eventsByDay[day][i]).Before(getEventStartTime(eventsByDay[day][j]))
		})
	}

	header := container.New(layout.NewGridLayout(7))
	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	for _, dayName := range dayNames {
		header.Add(widget.NewLabelWithStyle(dayName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))
	}

	grid := container.New(layout.NewGridLayout(7))
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < int(firstOfMonth.Weekday()); i++ {
		grid.Add(container.NewMax())
	}

	for day := 1; day <= daysInMonth(currentYear, currentMonth); day++ {
		dayLabel := widget.NewLabel(fmt.Sprintf("%d", day))
		var eventWidgets []fyne.CanvasObject
		for _, event := range eventsByDay[day] {
			eventText := fmt.Sprintf("%s\n%s", getEventStartTime(event).Format(config.Google.Calendar.TimeFormat), event.Summary)
			eventLabel := widget.NewLabel(eventText)
			eventLabel.Wrapping = fyne.TextWrapWord
			eventLabel.Alignment = fyne.TextAlignLeading
			border := canvas.NewRectangle(color.NRGBA{R: 100, G: 100, B: 100, A: 50})
			border.StrokeColor = theme.SeparatorColor()
			border.StrokeWidth = 1
			eventWidgets = append(eventWidgets, container.NewMax(border, container.NewVBox(eventLabel)))
		}

		dayBoxContent := container.NewVScroll(container.New(&ui.NoPaddingLayout{}, eventWidgets...))

		dayBorder := canvas.NewRectangle(color.Transparent)                 // Transparent background
		dayBorder.StrokeColor = color.NRGBA{R: 211, G: 211, B: 211, A: 255} // Light grey
		dayBorder.StrokeWidth = 1

		// Create empty spacers for left/right to ensure content is inset for border
		// This will be used in the container.NewBorder for the dayBoxContainer
		borderSpace := canvas.NewRectangle(color.Transparent)
		borderSpace.SetMinSize(fyne.NewSize(dayBorder.StrokeWidth, 0)) // 1 pixel width

		dayBoxContainerWithInset := container.NewBorder(
			dayLabel,      // Top: Day number
			nil,           // Bottom
			borderSpace,   // Left: Small spacer for border
			borderSpace,   // Right: Small spacer for border
			dayBoxContent, // Center: Event content
		)

		grid.Add(container.NewMax(dayBorder, dayBoxContainerWithInset))
	}

	monthYearLabel := widget.NewLabelWithStyle(fmt.Sprintf("%s %d", currentMonth, currentYear), fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	scrollableContent := container.NewScroll(newUnexpandingGrid(container.NewBorder(header, nil, nil, nil, grid)))
	return container.NewBorder(container.NewVBox(monthYearLabel, widget.NewSeparator()), nil, nil, nil, scrollableContent)
}

func CreateCalendarView(calService *gcalendar.Service) (fyne.CanvasObject, context.CancelFunc, func()) {
	if calService == nil {
		return container.NewCenter(widget.NewLabel("Google Calendar service not initialized.")), nil, func() {}
	}

	ctx, cancel := context.WithCancel(context.Background())

	currentCalendarDate = time.Now()
	cfg := config.GetConfig()
	var calendarContent *fyne.Container // Keep this local container for the navBar in the return

	updateContent := func() {
		// Immediately show a loading message while content is being prepared in a goroutine
		calendarContent.Objects = []fyne.CanvasObject{container.NewCenter(widget.NewLabel("Loading Calendar..."))}
		calendarContent.Refresh()

		go func() {
			var fetchedEvents []*gcalendar.Event
			var err error
			eventsInCache := false

			// Check if events for the current month are in the cache
			// This part can stay on the goroutine, it's just checking local cache
			for _, event := range CachedEvents {
				eventTime := getEventStartTime(event)
				if eventTime.Year() == currentCalendarDate.Year() && eventTime.Month() == currentCalendarDate.Month() {
					eventsInCache = true
					break
				}
			}

			// If not in cache or if cache is explicitly stale/empty, fetch them
			if !eventsInCache { // This logic was previously `if !eventsInCache`
				log.Printf("Fetching events for %s", currentCalendarDate.Format("Jan 2006"))
				fetchedEvents, err = GetEventsForMonth(calService, cfg.Google.Calendar, currentCalendarDate)
				if err != nil {
					log.Printf("Failed to fetch calendar events: %v", err)
					fyne.Do(func() {
						calendarContent.Objects = []fyne.CanvasObject{container.NewCenter(widget.NewLabel("Error fetching calendar events. Please check connectivity and configuration."))}
						calendarContent.Refresh()
					})
					SetCachedEvents(nil) // Clear cache on error
					return
				}
				SetCachedEvents(fetchedEvents) // Update global cache
				NotifyEventsUpdated()          // Notify other listeners if needed
			} else {
				fetchedEvents = CachedEvents // Use cached events
			}

			// Generate the new content, which can be computationally intensive
			var newContent fyne.CanvasObject
			if calendarViewMode == "month" {
				newContent = generateCalendarGrid(currentCalendarDate, calService, cfg)
			} else {
				newContent = generateWeekGrid(ctx, currentCalendarDate, calService, cfg)
			}

			// Update UI on the main thread once content is ready
			fyne.Do(func() {
				calendarContent.Objects = []fyne.CanvasObject{newContent}
				calendarContent.Refresh()
			})
		}()
	}

	// Initialize the local calendarContent for the navBar
	calendarContent = container.NewMax()
	updateContent() // Initial update to populate globalContentContainer

	// Listen for calendar event updates and refresh the view if open
	go func() {
		defer log.Println("Calendar events updated listener goroutine terminated.")
		for {
			select {
			case <-ctx.Done():
				return
			case <-CalendarEventsUpdatedChan: // Now listening to local channel
				log.Println("Calendar events updated, refreshing calendar view.")
				fyne.Do(updateContent)
			}
		}
	}()

	prevButton := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if calendarViewMode == "month" {
			currentCalendarDate = currentCalendarDate.AddDate(0, -1, 0)
		} else {
			currentCalendarDate = currentCalendarDate.AddDate(0, 0, -7)
		}
		updateContent()
	})

	nextButton := widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		if calendarViewMode == "month" {
			currentCalendarDate = currentCalendarDate.AddDate(0, 1, 0)
		} else {
			currentCalendarDate = currentCalendarDate.AddDate(0, 0, 7)
		}
		updateContent()
	})

	toggleButton := widget.NewButton("Week View", nil)
	toggleButton.OnTapped = func() {
		if calendarViewMode == "month" {
			calendarViewMode = "week"
			toggleButton.SetText("Month View")
		} else {
			calendarViewMode = "month"
			toggleButton.SetText("Week View")
		}
		updateContent()
	}

	navBar := container.NewHBox(layout.NewSpacer(), prevButton, toggleButton, nextButton, layout.NewSpacer())
	returnedContent := container.NewBorder(nil, navBar, nil, nil, calendarContent)
	return returnedContent, cancel, updateContent
}
