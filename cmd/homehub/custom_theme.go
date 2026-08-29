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

package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type myTheme struct {
	fyne.Theme
}

func (t *myTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return 12.0 // Increased font size by 50%
	}
	return t.Theme.Size(name)
}

func (t *myTheme) Color(
	name fyne.ThemeColorName, variant fyne.ThemeVariant,
) color.Color {
	switch name {
	case theme.ColorNameButton:
		return color.Black // Unselected button background
	case theme.ColorNamePrimary:
		return color.NRGBA{B: 255, A: 255} // Selected button background (blue)
	case theme.ColorNameForeground:
		return color.White // Icon/Text color for theme-aware elements
	case theme.ColorNameBackground:
		return color.Black // Ensure consistent dark background
	}

	return t.Theme.Color(name, variant)
}

func (t *myTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.Theme.Font(style)
}

func (t *myTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.Theme.Icon(name)
}

func (t *myTheme) Padding() float32 {
	return 0.0 // Remove all padding
}
