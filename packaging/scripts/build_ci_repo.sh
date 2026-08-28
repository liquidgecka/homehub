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

# Import GPG private key if provided in environment
if [ -n "$APT_GPG_PRIVATE_KEY" ]; then
    echo "Importing GPG private key..."
    echo "$APT_GPG_PRIVATE_KEY" | gpg --import --batch --yes
fi

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
if [ -f "${ROOT_DIR}/packaging/homehub.gpg" ]; then
    cp "${ROOT_DIR}/packaging/homehub.gpg" "$TARGET_DIR/"
fi

# 5. Regenerate APT indices
cd "$TARGET_DIR"
echo "homehub-apt.catherman.org" > CNAME

# Scan packages first so we can list available versions on the webpage
dpkg-scanpackages . /dev/null > Packages
gzip -k -f Packages

# Extract package versions from Packages
VERSIONS_HTML=""
if [ -f Packages ]; then
    CURRENT_VER=""
    CURRENT_ARCH=""
    CURRENT_FILE=""
    CURRENT_SIZE=""

    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in
            Version:\ *)
                CURRENT_VER="${line#Version: }"
                ;;
            Architecture:\ *)
                CURRENT_ARCH="${line#Architecture: }"
                ;;
            Filename:\ *)
                CURRENT_FILE="${line#Filename: }"
                ;;
            Size:\ *)
                SIZE_BYTES="${line#Size: }"
                if [ "$SIZE_BYTES" -ge 1048576 ] 2>/dev/null; then
                    CURRENT_SIZE="$(( SIZE_BYTES / 1048576 )).$(( (SIZE_BYTES % 1048576) * 10 / 1048576 )) MB"
                elif [ "$SIZE_BYTES" -ge 1024 ] 2>/dev/null; then
                    CURRENT_SIZE="$(( SIZE_BYTES / 1024 )) KB"
                else
                    CURRENT_SIZE="${SIZE_BYTES} B"
                fi
                ;;
            "")
                if [ -n "$CURRENT_VER" ] && [ -n "$CURRENT_FILE" ]; then
                    VERSIONS_HTML="${VERSIONS_HTML}
                <div class=\"version-item\">
                    <div class=\"version-info\">
                        <span class=\"version-tag\">v${CURRENT_VER}</span>
                        <span class=\"version-arch\">${CURRENT_ARCH}</span>
                        <span class=\"version-size\">${CURRENT_SIZE}</span>
                    </div>
                    <a href=\"${CURRENT_FILE}\" class=\"version-download\" download>Download .deb ⬇</a>
                </div>"
                    CURRENT_VER=""
                    CURRENT_ARCH=""
                    CURRENT_FILE=""
                    CURRENT_SIZE=""
                fi
                ;;
        esac
    done < Packages
    if [ -n "$CURRENT_VER" ] && [ -n "$CURRENT_FILE" ]; then
        VERSIONS_HTML="${VERSIONS_HTML}
                <div class=\"version-item\">
                    <div class=\"version-info\">
                        <span class=\"version-tag\">v${CURRENT_VER}</span>
                        <span class=\"version-arch\">${CURRENT_ARCH}</span>
                        <span class=\"version-size\">${CURRENT_SIZE}</span>
                    </div>
                    <a href=\"${CURRENT_FILE}\" class=\"version-download\" download>Download .deb ⬇</a>
                </div>"
    fi
fi

if [ -z "$VERSIONS_HTML" ]; then
    VERSIONS_HTML="<p style=\"color: var(--text-muted); font-size: 0.95rem; margin-top: 10px;\">No package versions found.</p>"
fi

# Generate dynamic index.html page
cat <<HTML > index.html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HomeHub APT Repository</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;800&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0b0f19;
            --card-bg: rgba(255, 255, 255, 0.03);
            --border-color: rgba(255, 255, 255, 0.08);
            --accent-color: #3b82f6;
            --accent-hover: #60a5fa;
            --text-color: #f3f4f6;
            --text-muted: #9ca3af;
        }
        body {
            margin: 0;
            padding: 0;
            background-color: var(--bg-color);
            color: var(--text-color);
            font-family: 'Outfit', sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            overflow-x: hidden;
        }
        .background-glow {
            position: absolute;
            width: 600px;
            height: 600px;
            background: radial-gradient(circle, rgba(59, 130, 246, 0.08) 0%, rgba(0,0,0,0) 70%);
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            z-index: -1;
            pointer-events: none;
        }
        .container {
            width: 100%;
            max-width: 650px;
            padding: 40px 20px;
            box-sizing: border-box;
        }
        .card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 24px;
            padding: 40px;
            backdrop-filter: blur(12px);
            box-shadow: 0 20px 40px rgba(0, 0, 0, 0.3);
            animation: fadeIn 0.8s ease-out;
        }
        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(20px); }
            to { opacity: 1; transform: translateY(0); }
        }
        h1 {
            font-size: 2.5rem;
            font-weight: 800;
            margin-top: 0;
            margin-bottom: 10px;
            background: linear-gradient(135deg, #fff 0%, #9ca3af 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            letter-spacing: -0.02em;
        }
        p {
            font-size: 1.1rem;
            line-height: 1.6;
            color: var(--text-muted);
            margin-bottom: 30px;
        }
        .section-title {
            font-size: 1.2rem;
            font-weight: 600;
            color: #fff;
            margin-top: 30px;
            margin-bottom: 15px;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .section-title::before {
            content: '';
            display: inline-block;
            width: 8px;
            height: 8px;
            background-color: var(--accent-color);
            border-radius: 50%;
        }
        pre {
            background: rgba(0, 0, 0, 0.4);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 16px 20px;
            font-family: 'Fira Code', 'Courier New', Courier, monospace;
            font-size: 0.95rem;
            overflow-x: auto;
            color: #38bdf8;
            margin: 0 0 20px 0;
            position: relative;
        }
        code {
            font-family: inherit;
        }
        .version-list {
            display: flex;
            flex-direction: column;
            gap: 10px;
            margin-top: 10px;
        }
        .version-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            background: rgba(0, 0, 0, 0.4);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 12px 18px;
            transition: border-color 0.2s, background-color 0.2s;
        }
        .version-item:hover {
            border-color: rgba(59, 130, 246, 0.4);
            background: rgba(59, 130, 246, 0.04);
        }
        .version-info {
            display: flex;
            align-items: center;
            gap: 12px;
            flex-wrap: wrap;
        }
        .version-tag {
            font-weight: 700;
            color: #f3f4f6;
            font-size: 1rem;
            font-family: 'Fira Code', 'Courier New', Courier, monospace;
        }
        .version-arch {
            font-size: 0.8rem;
            font-weight: 600;
            color: #60a5fa;
            background: rgba(59, 130, 246, 0.15);
            padding: 2px 8px;
            border-radius: 6px;
        }
        .version-size {
            font-size: 0.85rem;
            color: var(--text-muted);
        }
        .version-download {
            color: #38bdf8;
            text-decoration: none;
            font-size: 0.88rem;
            font-weight: 600;
            padding: 6px 12px;
            border-radius: 8px;
            background: rgba(56, 189, 248, 0.1);
            border: 1px solid rgba(56, 189, 248, 0.2);
            transition: all 0.2s;
            white-space: nowrap;
        }
        .version-download:hover {
            background: rgba(56, 189, 248, 0.25);
            color: #ffffff;
        }
        .footer {
            margin-top: 40px;
            text-align: center;
            font-size: 0.85rem;
            color: rgba(255, 255, 255, 0.2);
            letter-spacing: 0.05em;
        }
        .accent-text {
            color: var(--accent-color);
        }
    </style>
</head>
<body>
    <div class="background-glow"></div>
    <div class="container">
        <div class="card">
            <h1>HomeHub <span class="accent-text">APT</span> Repo</h1>
            <p>Welcome to the Debian package repository for HomeHub. Configure this repository on your touchscreen Ubuntu/Debian client machines to receive automatic updates.</p>
            
            <div class="section-title">1. Download Key</div>
            <pre><code>sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://homehub-apt.catherman.org/homehub.gpg | sudo gpg --dearmor -o /etc/apt/keyrings/homehub.gpg</code></pre>
            
            <div class="section-title">2. Add Repository Configuration</div>
            <pre><code>cat &lt;&lt;EOF | sudo tee /etc/apt/sources.list.d/homehub.sources
Types: deb
URIs: https://homehub-apt.catherman.org/
Suites: /
Signed-By: /etc/apt/keyrings/homehub.gpg
EOF</code></pre>
            
            <div class="section-title">3. Install HomeHub</div>
            <pre><code>sudo apt-get update
sudo apt-get install homehub</code></pre>

            <div class="section-title">4. Available Versions</div>
            <div class="version-list">
${VERSIONS_HTML}
            </div>
        </div>
        <div class="footer">
            &copy; 2026 BRADY CATHERMAN &bull; LICENSED UNDER APACHE 2.0
        </div>
    </div>
</body>
</html>
HTML

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

# If GPG key is imported, sign the Release file
if gpg --list-keys "HomeHub APT Repository" >/dev/null 2>&1; then
    echo "Signing Release metadata..."
    gpg --batch --yes --default-key "HomeHub APT Repository" -abs -o Release.gpg Release
    gpg --batch --yes --default-key "HomeHub APT Repository" --clearsign -o InRelease Release
else
    echo "Warning: GPG key 'HomeHub APT Repository' not found. Repository will be unsigned."
fi

echo "APT repository index regenerated successfully in $TARGET_DIR."
