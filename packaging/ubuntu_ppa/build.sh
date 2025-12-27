#!/bin/bash -e

check_installed_binary() {
    if [ -z "$(which "$1")" ] ; then
        echo "$1 is a required tool."
        exit 1
    fi
}

check_installed_binary debuild
check_installed_binary git
check_installed_binary gzip
check_installed_binary go
check_installed_binary tar

# Calculate the root of the repo.
ROOT="$( cd "$(dirname $0)/../.." && pwd )"

# Extract the previously generated orig.tar.gz file into /tmp/build.
version="$(cat /tmp/build/version)"
tar -C /tmp/build -zxf "/tmp/build/homehub_${version}.orig.tar.gz"

DEST="/tmp/build/homehub-${version}"
export GOPATH="$DEST"

# Prep the debian directory.
cp -r "${ROOT}/packaging/debian" "${DEST}/debian"
cp -r "${ROOT}/packaging/debian/control.ubuntu-$(lsb_release -rs)" \
       "${DEST}/debian/control"
cp /tmp/build/changelog "${DEST}/debian/changelog"
sed -i "s/__DISTRO__/$(lsb_release -cs)/g" "${DEST}/debian/changelog"
sed -i "s/VERSION/$version/" "${DEST}/debian/rules"

cd "/tmp/build/homehub-${version}"
debuild -S
