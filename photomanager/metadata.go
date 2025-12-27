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

package photomanager

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// GetCreationDate reads the EXIF data from an image file and returns the date it was taken.
var GetCreationDate = func(filePath string) (time.Time, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		// If there's no EXIF data, we can fall back to the file's modification time.
		info, statErr := os.Stat(filePath)
		if statErr != nil {
			return time.Time{}, statErr
		}
		return info.ModTime(), nil
	}

	// DateTimeOriginal is the preferred tag for when the photo was taken.
	dt, err := x.DateTime()
	if err != nil {
		// If DateTimeOriginal is not present, fall back to file's modification time.
		info, statErr := os.Stat(filePath)
		if statErr != nil {
			return time.Time{}, statErr
		}
		return info.ModTime(), nil
	}

	return dt, nil
}
