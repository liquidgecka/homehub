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

package weather

import (
	"fmt"
	"image/color"
	"log"
	"os"            // Added for os.Stat
	"path/filepath" // Added for filepath.Join
	"time"          // Re-added for time.Unix

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/liquidgecka/homehub/config"
)

func newSizedText(text string, size float32) *canvas.Text {
	return &canvas.Text{
		Text:     text,
		TextSize: size,
		Color:    color.White,
	}
}

func CreateView(cfg *config.Config) fyne.CanvasObject {
	weatherData, err := GetWeather(cfg)
	if err != nil {
		log.Printf("Failed to get weather data: %v", err)
		return container.NewCenter(widget.NewLabel("Loading weather..."))
	}

	current := weatherData.Current
	// daily is removed, its usage embedded directly below

	tempUnit := "°F"
	speedUnit := "mph"
	if cfg.App.WeatherUnits == "metric" {
		tempUnit = "°C"
		speedUnit = "m/s"
	}

	// --- Today's Weather ---
	todayIconSize := float32(256) // Larger icon for today
	currentIconCode := current.Weather[0].Icon
	if currentIconCode == "50d" || currentIconCode == "50n" {
		log.Println(
			"Overriding icon 50d/50n with 04d for current weather.",
		)
		currentIconCode = "04d"
	}
	todayIcon := getWeatherIconResource(cfg, currentIconCode, todayIconSize)

	// --- 7-Day Forecast ---
	forecastIconSize := float32(128) // Smaller icons for forecast
	forecastColumns := make([]fyne.CanvasObject, 0)

	for i, day := range weatherData.Daily { // Use weatherData.Daily directly
		if i == 0 { // Skip today, as it's covered by current weather
			continue
		}
		if i > 7 { // Limit to 7 days
			break
		}
		date := time.Unix(day.Dt, 0).Format("Mon, Jan 2")
		dayIcon := getWeatherIconResource(
			cfg, day.Weather[0].Icon, forecastIconSize,
		)

		forecastColumn := container.NewVBox(
			newSizedText(date, 20),
			dayIcon,
			newSizedText(
				fmt.Sprintf("High: %.0f%s", day.Temp.Max, tempUnit), 18,
			),
			newSizedText(
				fmt.Sprintf("Low: %.0f%s", day.Temp.Min, tempUnit), 18,
			),
			newSizedText(day.Weather[0].Description, 18),
		)
		forecastColumns = append(forecastColumns, forecastColumn)
	}

	tempText := fmt.Sprintf(
		"Temperature: %.1f%s (Feels like: %.1f%s)",
		current.Temp, tempUnit, current.FeelsLike, tempUnit,
	)
	condText := fmt.Sprintf(
		"Condition: %s (%s)",
		current.Weather[0].Main, current.Weather[0].Description,
	)
	windText := fmt.Sprintf(
		"Wind: %.1f %s", current.WindSpeed, speedUnit,
	)

	return container.NewBorder(container.NewVBox(
		newSizedText(
			fmt.Sprintf("Current weather in %s", cfg.OpenWeather.Location), 28,
		),
		container.NewHBox( // Use HBox for icon and text
			todayIcon,
			container.NewVBox(
				newSizedText(tempText, 20),
				newSizedText(condText, 20),
				newSizedText(fmt.Sprintf("Humidity: %d%%", current.Humidity), 20),
				newSizedText(windText, 20),
			),
		),
		widget.NewSeparator(),
	), nil, nil, nil, container.NewVBox( // Embed forecastContent directly
		newSizedText("7-Day Forecast", 24),
		container.NewGridWithColumns(7, forecastColumns...),
	))
}

// getWeatherIconResource creates a Fyne image resource from OpenWeatherMap icon
// code, preferring cached local files.
func getWeatherIconResource(
	cfg *config.Config, iconCode string, size float32,
) fyne.CanvasObject {
	localFilePath := filepath.Join(
		cfg.OpenWeather.ImageCacheDir,
		fmt.Sprintf("%s@4x.png", iconCode),
	)

	// Check if the image exists in the local cache
	if _, err := os.Stat(localFilePath); err == nil {
		img := canvas.NewImageFromFile(localFilePath)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(size, size))
		return img
	}

	// If not found in cache, log and return fallback.
	log.Printf(
		"Weather icon %s not found in cache %s. Fallback.",
		iconCode, localFilePath,
	)
	return canvas.NewCircle(color.White) // Fallback
}
