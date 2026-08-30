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
// See the License for the specific language governing permissions and
// limitations under the License.

package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/liquidgecka/homehub/config"
)

var (
	// Updated to OpenWeatherMap One Call API 3.0
	openWeatherMapAPIURL = "https://api.openweathermap.org/data/3.0"
	geocodingAPIURL      = "https://api.openweathermap.org/geo/1.0"

	// downloadImageFunc is package-level variable so it can be mocked.
	downloadImageFunc = downloadImage
)

var httpClient = http.DefaultClient

// SetMockHTTPClient sets a mock HTTP client for testing purposes.
func SetMockHTTPClient(client *http.Client) {
	httpClient = client
}

// RestoreHTTPClient restores the default HTTP client.
func RestoreHTTPClient() {
	httpClient = http.DefaultClient
}

// GeocodingResponse represents response from OpenWeatherMap Geocoding API.
type GeocodingResponse []struct {
	Name       string `json:"name"`
	LocalNames struct {
		En string `json:"en"`
	} `json:"local_names"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Country string  `json:"country"`
	State   string  `json:"state"`
}

// OpenWeather represents structure for OpenWeatherMap One Call API response.
type OpenWeather struct {
	Lat            float64        `json:"lat"`
	Lon            float64        `json:"lon"`
	Timezone       string         `json:"timezone"`
	TimezoneOffset int            `json:"timezone_offset"`
	Current        CurrentWeather `json:"current"`
	Daily          []DailyWeather `json:"daily"`
}

// CurrentWeather represents current weather conditions.
type CurrentWeather struct {
	Dt         int64                `json:"dt"`
	Sunrise    int64                `json:"sunrise"`
	Sunset     int64                `json:"sunset"`
	Temp       float64              `json:"temp"`
	FeelsLike  float64              `json:"feels_like"`
	Pressure   int                  `json:"pressure"`
	Humidity   int                  `json:"humidity"`
	DewPoint   float64              `json:"dew_point"`
	Uvi        float64              `json:"uvi"`
	Clouds     int                  `json:"clouds"`
	Visibility int                  `json:"visibility"`
	WindSpeed  float64              `json:"wind_speed"`
	WindDeg    int                  `json:"wind_deg"`
	Weather    []WeatherDescription `json:"weather"`
}

// DailyWeather represents daily forecast data.
type DailyWeather struct {
	Dt        int64                `json:"dt"`
	Sunrise   int64                `json:"sunrise"`
	Sunset    int64                `json:"sunset"`
	Temp      DailyTemp            `json:"temp"`
	FeelsLike DailyFeelsLike       `json:"feels_like"`
	Pressure  int                  `json:"pressure"`
	Humidity  int                  `json:"humidity"`
	DewPoint  float64              `json:"dew_point"`
	WindSpeed float64              `json:"wind_speed"`
	WindDeg   int                  `json:"wind_deg"`
	Weather   []WeatherDescription `json:"weather"`
	Clouds    int                  `json:"clouds"`
	Pop       float64              `json:"pop"`
	Uvi       float64              `json:"uvi"`
}

// DailyTemp represents daily temperature details.
type DailyTemp struct {
	Day   float64 `json:"day"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Night float64 `json:"night"`
	Eve   float64 `json:"eve"`
	Morn  float64 `json:"morn"`
}

// DailyFeelsLike represents daily "feels like" temperatures.
type DailyFeelsLike struct {
	Day   float64 `json:"day"`
	Night float64 `json:"night"`
	Eve   float64 `json:"eve"`
	Morn  float64 `json:"morn"`
}

// WeatherDescription represents weather conditions (e.g., "clear sky", "rain").
type WeatherDescription struct {
	ID          int    `json:"id"`
	Main        string `json:"main"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// WeatherCache stores cached weather data and the time it was last fetched.
type WeatherCache struct {
	data        *OpenWeather
	lastFetched time.Time
	mutex       sync.RWMutex
}

var weatherCache = &WeatherCache{}

// downloadImage fetches a file from a URL and saves it to specified filePath.
func downloadImage(url, filePath string) error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download image from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"failed to download image from %s, status: %d",
			url, resp.StatusCode,
		)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save image to %s: %w", filePath, err)
	}
	return nil
}

// GetCoordinates fetches latitude and longitude for a given city name.
var GetCoordinates = func(
	cityName string, apiKey string,
) (float64, float64, error) {
	geocodingURL := fmt.Sprintf(
		"%s/direct?q=%s&limit=1&appid=%s",
		geocodingAPIURL, url.QueryEscape(cityName), apiKey,
	)

	resp, err := httpClient.Get(geocodingURL)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch geocoding data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf(
			"geocoding API request failed with status %d", resp.StatusCode,
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf(
			"failed to read geocoding response body: %w", err,
		)
	}

	var geoResp GeocodingResponse
	if err := json.Unmarshal(body, &geoResp); err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal geocoding response: %w", err)
	}

	if len(geoResp) == 0 {
		return 0, 0, fmt.Errorf("no coordinates found for city: %s", cityName)
	}

	return geoResp[0].Lat, geoResp[0].Lon, nil
}

// GetWeather fetches current and 7-day forecast data, with caching.
var GetWeather = func(cfg *config.Config) (*OpenWeather, error) {
	weatherCache.mutex.RLock()
	// Check cache validity
	if weatherCache.data != nil &&
		time.Since(weatherCache.lastFetched).Minutes() <
			float64(cfg.OpenWeather.RefreshMinutes) {
		defer weatherCache.mutex.RUnlock()
		return weatherCache.data, nil
	}
	weatherCache.mutex.RUnlock()

	// Cache is expired or empty, fetch new data
	lat, lon, err := GetCoordinates(
		cfg.OpenWeather.Location, cfg.OpenWeather.APIKey,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get coordinates for %s: %w",
			cfg.OpenWeather.Location, err,
		)
	}

	// Fetch weather data using One Call API
	units := "imperial" // Default to imperial if not set or invalid
	if cfg.App.WeatherUnits == "metric" {
		units = "metric"
	}

	oneCallURL := fmt.Sprintf(
		"%s/onecall?lat=%f&lon=%f&exclude=minutely,hourly&appid=%s&units=%s",
		openWeatherMapAPIURL, lat, lon, cfg.OpenWeather.APIKey, units,
	)

	resp, err := httpClient.Get(oneCallURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"weather API request failed with status %d: %s",
			resp.StatusCode, string(bodyBytes),
		)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read weather response body: %w", err)
	}

	var weatherData OpenWeather
	if err := json.Unmarshal(body, &weatherData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal weather data: %w", err)
	}

	// Download and cache weather icons
	downloadedIcons := make(map[string]bool)
	iconsToDownload := []string{}

	for _, wDesc := range weatherData.Current.Weather {
		if _, ok := downloadedIcons[wDesc.Icon]; !ok {
			iconsToDownload = append(iconsToDownload, wDesc.Icon)
			downloadedIcons[wDesc.Icon] = true
		}
	}
	for _, dailyData := range weatherData.Daily {
		for _, wDesc := range dailyData.Weather {
			if _, ok := downloadedIcons[wDesc.Icon]; !ok {
				iconsToDownload = append(iconsToDownload, wDesc.Icon)
				downloadedIcons[wDesc.Icon] = true
			}
		}
	}

	for _, iconCode := range iconsToDownload {
		sanitizedCode := filepath.Base(filepath.Clean(iconCode))
		if sanitizedCode == "" || sanitizedCode == "." ||
			sanitizedCode == ".." {
			continue
		}
		imageURL := fmt.Sprintf(
			"https://openweathermap.org/img/wn/%s@4x.png", sanitizedCode,
		)
		localFilePath := filepath.Join(
			cfg.OpenWeather.ImageCacheDir,
			fmt.Sprintf("%s@4x.png", sanitizedCode),
		)

		if _, err := os.Stat(localFilePath); os.IsNotExist(err) {
			log.Printf(
				"Downloading weather icon %s to %s",
				imageURL, localFilePath,
			)
			if err := downloadImageFunc(imageURL, localFilePath); err != nil {
				log.Printf("Error downloading icon %s: %v", imageURL, err)
			}
		} else if err != nil {
			log.Printf(
				"Error checking icon cache for %s: %v",
				localFilePath, err,
			)
		}
	}

	// Update cache
	weatherCache.mutex.Lock()
	weatherCache.data = &weatherData
	weatherCache.lastFetched = time.Now()
	weatherCache.mutex.Unlock()

	return &weatherData, nil
}
