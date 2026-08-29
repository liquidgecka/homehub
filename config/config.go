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

package config

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var (
	// activeConfigPath stores the path of the currently loaded config file.
	// This is set by LoadConfig and used by SaveConfig.
	activeConfigPath string
)

// GetDefaultConfigPath returns the default path for the configuration file.
func GetDefaultConfigPath() string {
	usr, err := user.Current()
	if err != nil {
		log.Printf("Error getting user's home directory: %v", err)
		// Fallback to a relative path if home directory is not available
		return "config.toml"
	}
	return filepath.Join(usr.HomeDir, ".local", "homehub", "config.toml")
}

// Config holds all application configuration settings.
type Config struct {
	App         AppConfig         `toml:"app"`
	LocalPhotos LocalPhotosConfig `toml:"local_photos"`
	Google      GoogleConfig      `toml:"google"`
	OpenWeather OpenWeatherConfig `toml:"openweathermap"`
	Shopping    ShoppingConfig    `toml:"shopping"`
	Finance     FinanceConfig     `toml:"finance"`
	DPMS        DPMSConfig        `toml:"dpms"`
	Security    SecurityConfig    `toml:"security"`
	Logging     LoggingConfig     `toml:"logging"`
	Database    DatabaseConfig    `toml:"database"`
}

// AppConfig holds general application settings.
type AppConfig struct {
	IdleTimeoutMinutes          int    `toml:"idle_timeout_minutes"`
	WebServerPort               int    `toml:"web_server_port"`
	WebServerListenAddress      string `toml:"web_server_listen_address"`
	WeatherUnits                string `toml:"weather_units"`
	IconsDirectory              string `toml:"icons_directory"`
	HideMouseCursorOnX11Startup bool   `toml:"hide_mouse_cursor_on_x11_startup"`
	WebTemplatesDirectory       string `toml:"web_templates_directory"`
	OnscreenKeyboardCommand     string `toml:"onscreen_keyboard_command"`
}

// LocalPhotosConfig holds settings for the local photo viewer.
type LocalPhotosConfig struct {
	Directory               string `toml:"directory"`
	RotationIntervalSeconds int    `toml:"rotation_interval_seconds"`
}

// GoogleConfig holds Google API related settings.
type GoogleConfig struct {
	ServiceAccountKeyFile string               `toml:"service_account_key_file"`
	Calendar              GoogleCalendarConfig `toml:"calendar"`
}

// GoogleCalendarConfig holds Google Calendar specific settings.
type GoogleCalendarConfig struct {
	CalendarIDs            []string `toml:"calendar_ids"`
	CalendarRefreshMinutes int      `toml:"calendar_refresh_minutes"`
	TimeFormat             string   `toml:"time_format"`
}

// OpenWeatherConfig holds OpenWeatherMap specific settings.
type OpenWeatherConfig struct {
	APIKey         string `toml:"api_key"`
	Location       string `toml:"location"`
	RefreshMinutes int    `toml:"refresh_minutes"`
	ImageCacheDir  string `toml:"image_cache_dir"`
}

// ShoppingConfig holds shopping list specific settings.
type ShoppingConfig struct {
	Store         []StoreConfig     `toml:"store"`
	LogoDirectory string            `toml:"logo_directory"`
	GoogleTasks   GoogleTasksConfig `toml:"google_tasks"`
}

// GoogleTasksConfig holds the configuration for Google Tasks integration.
type GoogleTasksConfig struct {
	Enabled     bool              `toml:"enabled"`
	ListMapping map[string]string `toml:"list_mapping"`
}

// StoreConfig defines settings for a single shopping store.
type StoreConfig struct {
	Name     string `toml:"name"`
	Icon     string `toml:"icon"` // Path to icon file or icon name
	Disabled bool   `toml:"disabled,omitempty"`
}

// FinanceConfig holds financial ledger specific settings.
type FinanceConfig struct {
	CurrencyUnit string `toml:"currency_unit"` // e.g., "USD", "EUR"
}

// DPMSConfig holds DPMS specific settings.
type DPMSConfig struct {
	OnPeriods            [][2]string `toml:"on_periods"`
	OnCommand            []string    `toml:"on_command"`
	OffCommand           []string    `toml:"off_command"`
	CheckIntervalSeconds int         `toml:"check_interval_seconds"`
}

// SecurityConfig holds the configuration for the security camera view.
type SecurityConfig struct {
	Camera      []CameraConfig    `toml:"camera"`
	FrigateMQTT FrigateMqttConfig `toml:"frigate_mqtt"`
}

// MqttConfig holds MQTT broker details.
type FrigateMqttConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// CameraConfig holds the configuration for a single camera.
type CameraConfig struct {
	Name     string `toml:"name"`
	Type     string `toml:"type"`
	URL      string `toml:"url"`
	Refresh  string `toml:"refresh"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// LoggingConfig holds settings for log file output and auto-rotation.
type LoggingConfig struct {
	// Directory for log files (default: ~/.local/homehub/logs)
	Directory string `toml:"directory"`
	// Location is an alias for directory
	Location string `toml:"location"`
	// Rotation interval/size (e.g. "10M", "10MB", default: 10M)
	RotationInterval string `toml:"rotation_interval"`
	// Rotation size in MB (default: 10)
	RotationSizeMB int `toml:"rotation_size_mb"`
	// Number of rotated log files to retain (default: 10)
	RetentionCount int `toml:"retention_count"`
	// Alias for retention_count
	MaxBackups int `toml:"max_backups"`
	// Name of the active log file (default: homehub.log)
	Filename string `toml:"filename"`
	// Enable debug logging
	Debug bool `toml:"debug"`
	// Log level (e.g. "debug", "info")
	Level string `toml:"level"`
}

// DatabaseConfig holds settings for database file and automated backups.
type DatabaseConfig struct {
	// Directory for backup zip files (default: ~/.local/homehub/backups)
	BackupDirectory string `toml:"backup_directory"`
	// Backup interval in hours (default: 24)
	BackupIntervalHours int `toml:"backup_interval_hours"`
	// Number of days to retain backup files (default: 30)
	BackupRetentionDays int `toml:"backup_retention_days"`
	// Enabled controls whether scheduled backups run (default: true)
	BackupEnabled *bool `toml:"backup_enabled,omitempty"`
}

// IsBackupEnabled returns true if scheduled backups are enabled
// (default: true).
func (d DatabaseConfig) IsBackupEnabled() bool {
	if d.BackupEnabled == nil {
		return true
	}
	return *d.BackupEnabled
}

// AccountConfig defines settings for a single financial account/person.
type AccountConfig struct {
	ID             int     `toml:"id"` // Added for database integration
	Name           string  `toml:"name"`
	InitialBalance float64 `toml:"initial_balance"`
	CurrentBalance float64 `toml:"current_balance"`
}

var _typedCfg Config // Stores the typed configuration

// DefaultConfig returns a Config struct initialized with default values.
func DefaultConfig() Config {
	return Config{
		App: AppConfig{
			IdleTimeoutMinutes:          5,
			WebServerPort:               8080,
			WebServerListenAddress:      "0.0.0.0",
			WeatherUnits:                "imperial",
			IconsDirectory:              "/usr/share/homehub/icons",
			HideMouseCursorOnX11Startup: false,
			WebTemplatesDirectory:       "/usr/share/homehub/web_templates",
			OnscreenKeyboardCommand:     "",
		},
		LocalPhotos: LocalPhotosConfig{
			Directory: filepath.Join(
				os.Getenv("HOME"), ".local", "share", "homehub", "photos",
			),
			RotationIntervalSeconds: 10,
		},
		Google: GoogleConfig{
			Calendar: GoogleCalendarConfig{
				TimeFormat:             "3 PM",
				CalendarRefreshMinutes: 5,
			},
		},
		OpenWeather: OpenWeatherConfig{
			APIKey:         "YOUR_OPENWEATHERMAP_API_KEY",
			Location:       "London,UK",
			RefreshMinutes: 15,
			ImageCacheDir: filepath.Join(
				os.Getenv("HOME"), ".cache", "homehub", "weather_photos",
			),
		},
		Shopping: ShoppingConfig{
			LogoDirectory: "/usr/share/homehub/icons",
		},
		DPMS: DPMSConfig{
			OnPeriods: [][2]string{{"07:00", "22:00"}},
			OnCommand: []string{
				"xset", "-display", ":0", "dpms", "force", "on",
			},
			OffCommand: []string{
				"xset", "-display", ":0", "dpms", "force", "off",
			},
			CheckIntervalSeconds: 15,
		},
		Logging: LoggingConfig{
			Directory: filepath.Join(
				os.Getenv("HOME"), ".local", "homehub", "logs",
			),
			RotationSizeMB: 10,
			RetentionCount: 10,
			Filename:       "homehub.log",
		},
		Database: DatabaseConfig{
			BackupDirectory: filepath.Join(
				os.Getenv("HOME"), ".local", "homehub", "backups",
			),
			BackupIntervalHours: 24,
			BackupRetentionDays: 30,
		},
	}
}

// LoadConfig reads the configuration from the specified TOML file.
func LoadConfig(configPath string) error {
	_typedCfg = DefaultConfig() // Initialize with defaults
	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf(
				"Config file '%s' not found. Using default config.",
				configPath,
			)
			// Store the path even if it doesn't exist yet
			activeConfigPath = configPath
			return err
		}
		return err
	}
	defer f.Close()

	meta, err := toml.NewDecoder(f).Decode(&_typedCfg)
	if err != nil {
		return err
	}
	if undecodedKeys := meta.Undecoded(); len(undecodedKeys) > 0 {
		return fmt.Errorf("unknown config fields: %v", undecodedKeys)
	}

	// On successful load, store the path.
	activeConfigPath = configPath
	return nil
}

// SaveConfig writes the given configuration to the active config file path
// and updates the in-memory global config.
var SaveConfig = func(cfg *Config) error {
	// Update _typedCfg with the new values. This is the authoritative state.
	_typedCfg = *cfg

	// Ensure activeConfigPath is not empty
	if activeConfigPath == "" {
		return fmt.Errorf("config file path is not set, cannot save config")
	}

	// Only write to disk if not running in a test environment
	if flag.Lookup("test.v") == nil {
		// Ensure the directory exists
		dir := filepath.Dir(activeConfigPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf(
				"unable to create config directory '%s': %w", dir, err,
			)
		}

		f, err := os.OpenFile(
			activeConfigPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644,
		)
		if err != nil {
			return fmt.Errorf(
				"unable to open config file for writing: %w", err,
			)
		}
		defer f.Close()

		if err := toml.NewEncoder(f).Encode(_typedCfg); err != nil {
			return fmt.Errorf("unable to encode config to TOML: %w", err)
		}
	} else {
		log.Printf("SaveConfig: Skipping disk write in test environment.")
	}

	return nil
}

// GetConfig returns the loaded configuration.
func GetConfig() *Config {
	return &_typedCfg
}

// SetMockConfig sets the global configuration for testing purposes.
func SetMockConfig(mockConfig Config) func() {
	originalConfig := _typedCfg
	_typedCfg = mockConfig
	return func() {
		_typedCfg = originalConfig
	}
}

func ValidateConfig() error {
	config := GetConfig()

	if config.App.IdleTimeoutMinutes == 0 {
		return fmt.Errorf(
			"app.idle_timeout_minutes cannot be 0. " +
				"Please set a value in config.toml",
		)
	}

	if config.LocalPhotos.Directory == "" {
		return fmt.Errorf(
			"local_photos.directory must be configured in config.toml",
		)
	}

	if config.Google.Calendar.CalendarRefreshMinutes == 0 {
		return fmt.Errorf(
			"google.calendar.calendar_refresh_minutes cannot be 0. " +
				"Please set a value in config.toml",
		)
	}

	if len(config.Google.Calendar.CalendarIDs) == 0 {
		return fmt.Errorf(
			"google.calendar.calendar_ids must be specified in config.toml",
		)
	}

	// Basic validation: Check if API keys are still placeholders
	if config.OpenWeather.APIKey == "YOUR_OPENWEATHERMAP_API_KEY" ||
		config.OpenWeather.APIKey == "" {
		return fmt.Errorf("OpenWeatherMap API Key is not configured")
	}
	if config.OpenWeather.Location == "" ||
		config.OpenWeather.Location == "London,UK" {
		log.Println(
			"Warning: OpenWeatherMap location is default 'London,UK'.",
		)
	}

	for _, camera := range config.Security.Camera {
		if camera.Type == "" {
			return fmt.Errorf(
				"camera '%s' must have a type specified", camera.Name,
			)
		}
	}

	return nil
}
