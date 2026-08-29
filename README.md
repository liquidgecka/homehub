# HomeHub

HomeHub is a smart home dashboard and family coordination application written
in Go. It is designed to run on a wall-mounted or desktop touchscreen-enabled
Ubuntu display, paired with an embedded companion web server accessible from
phones, tablets, and computers across the local network.

## Features

### 🖼️ Home Slideshow & Photo Management
- **Ambient Slideshow**: Displays rotating photos from your local library with
  customizable transition intervals and EXIF orientation awareness.
- **Interactive Controls**: Touchscreen heart icon to prioritize favorites
  (shown 2x more often) and thumbs-down icon to hide photos (automatically
  pruned after 30 days).
- **Photo Manager**: Searchable gallery with EXIF date/name filters, thumbnail
  browsing, and a full-size photo detail viewer.
- **Web Photo Uploader & Gallery**: Drag-and-drop batch photo uploading,
  sorting by EXIF/upload date/name, and storage space monitoring.

### 📅 Calendar & Events
- **Wall-Hung Calendar View**: Monthly grid showing events fetched from
  configured Google Calendars.
- **Chronological Sorting**: Daily event listings ordered by start time.
- **Service Account Integration**: Connects via Google Cloud Service Account
  without requiring user re-authentication.

### 🛒 Shopping Lists
- **Multi-Store Management**: Organize groceries and items by store with
  customizable store branding.
- **Interactive Checklists**: Real-time checkboxes, item quantities, and
  instant adding/editing.
- **Touchscreen Keyboard**: On-screen keyboard integration (`onboard`) for
  seamless touchscreen data entry.
- **Google Tasks Sync**: Optional bidirectional synchronization with Google
  Tasks.

### ☀️ Weather Forecast
- **Station-Style Weather**: Real-time current temperature, daily high/low,
  and animated condition icons powered by the OpenWeatherMap 3.0 API.
- **10-Day Forecast**: Extended forecast panels with icon caching for offline
  resilience.
- **Display Power Management (DPMS)**: Automatically dims or puts the screen
  into standby during night hours or inactivity.

### 💳 Financial Ledger
- **Multi-Account Ledgers**: Track savings funds, household expenses, and
  budgets across interactive tabs.
- **Transaction History**: Running expense records color-coded by credit and
  debit with running balance calculations.
- **Web & Desktop Interface**: Full tabbed interface with quick transaction
  entry and editing.

### ⏰ Reminders & Alerts
- **Household Reminders**: Schedule daily or weekly recurring reminders by
  time of day and active days of the week.
- **Slideshow Overlay**: Interactive alert banners overlaying the ambient
  slideshow when active reminders trigger.
- **Web & Touchscreen Controls**: Easily acknowledge, toggle, edit, or delete
  reminders.

### 📹 Security Cameras
- **Tiled Video Grid**: Monitor live Frigate RTSP/JPEG camera feeds with
  configurable refresh intervals and authentication.
- **Camera Focus**: Tap any camera to enlarge to full screen.
- **MQTT Event Tracking**: Real-time event subscription for Frigate object and
  motion detection events.

### 🌐 Web Companion Interface
- **Embedded Web Server**: Access HomeHub from any browser on the local
  network (default port `8080`).
- **Full Feature Parity**: Manage shopping lists, financial ledgers,
  reminders, and photo galleries with responsive tabbed layouts.
- **Database Backups & Recovery**: Automated scheduled zip backups (every 24h
  with 30-day retention), web-based restore, export, and disaster recovery.
- **System Controls**: Restart HomeHub in place or shut down gracefully
  directly from the web dashboard.

---

## Getting Started

### Prerequisites
- **Go**: 1.22 or newer.
- **Fyne Dependencies**: Graphical libraries for Linux/X11 (`libgl1-mesa-dev`,
  `xorg-dev`, etc.).
- **SQLite**: Embedded database for local persistence.

### Configuration
1. Copy the example configuration file:
   ```bash
   cp config.toml.example config.toml
   ```
2. Edit `config.toml` to configure your location, API keys, Google Cloud
   Service Account credentials, and local photo directory.
3. For Google Calendar and Google Tasks setup, follow the instructions in
   [`SETUP.md`](SETUP.md).

### Building & Running
To run tests and code cleanliness checks:
```bash
packaging/scripts/check
```

To build and run the application:
```bash
go build -o homehub ./cmd/homehub
./homehub
```

---

## Architecture & Code Structure

- `cmd/homehub/`: Main application entry point.
- `home/`: Slideshow manager and touchscreen home view.
- `photomanager/`: Photo loading, EXIF orientation decoding, thumbnail
  generation, and metadata database.
- `calendar/`: Google Calendar integration and monthly calendar view.
- `shopping/`: Shopping list database, Google Tasks synchronization, and
  touchscreen view.
- `weather/`: OpenWeatherMap client, caching, and weather view.
- `ledger/`: Financial ledger database models and touchscreen view.
- `reminders/`: Scheduled reminders engine, overlay views, and database.
- `security/`: Camera stream fetchers and Frigate MQTT event listener.
- `web/`: Embedded HTTP server and HTML templates for the companion web
  interface.
- `config/`: Application configuration loader and validation.
- `logging/`: Log management with automated file rotation and retention.
- `dpms/`: Display Power Management Signaling controller.

---

## License

All source files are licensed under the Apache 2.0 License.
Copyright 2026 - Brady Catherman.
