#!/bin/bash
#
# Copyright 2026 - Brady Catherman
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -e

if [ "$#" -lt 1 ]; then
    echo "Usage: $0 <target-repo-dir>"
    exit 1
fi

TARGET_DIR="$(readlink -f "$1")"
mkdir -p "$TARGET_DIR"

# 1. Calculate version name
# Commit date format: YYYY-MM-DD
COMMIT_DATE=$(git show -s --format=%cd --date=format:'%Y-%m-%d' HEAD)
YEAR=$(echo "$COMMIT_DATE" | cut -d'-' -f1 | cut -c3-4)
MONTH=$(echo "$COMMIT_DATE" | cut -d'-' -f2)
DAY=$(echo "$COMMIT_DATE" | cut -d'-' -f3)
VERSION_DATE="${YEAR}.${MONTH}.${DAY}"

# Number of commits authored on this specific day
COMMIT_COUNT=$(git log --format='%ad' --date=format:'%Y-%m-%d' | grep -c "^${COMMIT_DATE}$")
VERSION="${VERSION_DATE}-${COMMIT_COUNT}"

echo "Building Debian package version: ${VERSION}"

# 2. Setup build directory
ROOT_DIR="$(pwd -P)"
DEST="/tmp/build/homehub-${VERSION}"
rm -rf "$DEST"
mkdir -p "${DEST}/src/github.com/liquidgecka/homehub"

# Copy files
rsync -a --exclude=.git ./ "${DEST}/src/github.com/liquidgecka/homehub/"

# Copy packaging config files
cp -rf ./packaging/debian "${DEST}/debian"
cp -rf ./packaging/debian.conf "${DEST}/homehub.conf"
cp -rf ./packaging/logrotate.conf "${DEST}/homehub"
sed -i "s/VERSION/${VERSION}/" "${DEST}/debian/rules"

# Create changelog
cat << EOF > "${DEST}/debian/changelog"
homehub (${VERSION}-1) disco; urgency=low

  * Build version ${VERSION}.

 -- Brady Catherman <ubuntu@gecka.us>  $(date -R)
EOF

# 3. Build package
cd "${DEST}/src/github.com/liquidgecka/homehub"
go mod vendor
rm -f go.mod go.sum

cd "${DEST}"
export GOPATH="${DEST}"
export GO111MODULE=off
debuild -us -uc -b -d

# 4. Copy build result to target directory
cp /tmp/build/homehub_${VERSION}-*.deb "$TARGET_DIR/"

# 5. Regenerate APT indices
cd "$TARGET_DIR"
echo "homehub-apt.catherman.org" > CNAME
dpkg-scanpackages . /dev/null > Packages
gzip -k -f Packages

# Generate Release metadata
cat <<EOF > Release
Origin: HomeHub Repository
Label: HomeHub
Suite: stable
Codename: stable
Architectures: amd64 arm64 armhf
Components: main
Description: Static APT Repository for HomeHub
EOF
apt-ftparchive release . >> Release

echo "APT repository index regenerated successfully in $TARGET_DIR."
