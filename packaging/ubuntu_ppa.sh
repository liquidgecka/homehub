#!/bin/bash -e

export DEBEMAIL="ubuntu@gecka.us"
export DEBFULLNAME="Brady Catherman"

check_binary() {
    if [ -z "$(which "$1")" ] ; then
        echo "$1 is a required tool."
        exit 1
    fi
}

check_binary backportpackage
check_binary debuild
check_binary git
check_binary go
check_binary tar

# Calculate the root of the repo.
ROOT="$( cd "$(dirname $0)/.." && pwd )"

# Get the current git commit.
current_commit="$(git rev-parse HEAD)"

# If the current branch is dirty then refuse to build since it won't
# create a clean build source.
#if [ -n "$(git status --short)" ] ; then
#    echo "Refusing to build in dirty working branch."
#    exit 1
#fi

# Generate a list of version tags and store them in a bash hash.
declare -A hash_versions
while read hash tag ; do
    if [[ "$tag" =~ refs/tags/v[0-9a-z\.-]* ]] ; then
        version="${tag#refs/tags/v}"
        hash_versions["$hash"]="${version}"
    fi
done < <(
        git for-each-ref \
            --sort=-taggerdate \
            --format '%(objectname) %(refname)' \
            refs/tags
    )

# If the current commit does not match an expected tag then we are building
# on a commit that is not an official release. If this is not a tagged
# release then we need to make sure that the packages are not marked as
# in the release pocket.
release_flag="-r"
if [ -z "${hash_versions["$current_commit"]}" ] ; then
    echo "This commit is not tagged as a release, building packages but not"
    echo "marking it as releasable."
    release_flag=""
    hash_versions["$current_commit"]="0-test-only-release"
fi

# Now we walk through the changes in historical order (reversed from what
# git outputs, generating the change log file as best as is possible.
commits=()
version=""
dirty=()
found="no"
for commit in $(git log --format='%H') ; do
    commits+=("$(git show -s --format='%s' $commit)")
    # If this commit is tagged with a version then we can generate the
    # changelog entry for it.
    this_version="${hash_versions[$commit]}"
    if [[ -n "${this_version}" ]] ; then
        echo "homehub (${this_version}-1) disco; urgency=low"
        for message in "${commits}" ; do
            echo "  * ${message}"
        done
        date="$(git show -s --format="%cD" $commit)"
        echo " -- ${DEBFULLNAME} <${DEBEMAIL}>  ${date}"
        echo
        found="yes"
        commits=()
        if [ -z "$version" ] ; then
            version="$this_version"
        fi
    elif [ "$found" == no ] ; then
        dirty+=("$(git show -s --format='%s' $commit)")
    fi
done > "${ROOT}/packaging/debian/changelog"
if [ "${dirty}" == "y" ] ; then
    echo "${dirty} are still outstanding"
    echo "Repo is dirty. Can not build."
    exit 1
fi

set -x

# Setup some variables for use during the build.
DEST="/tmp/build/homehub-${version}"
export GOPATH="$DEST"


# Setup the build environment. Since most deb building services do not allow
# downloads we need to convert the modules into vendorized directories. This
# will allow the source to act as a single package rather than requiring
# every single dependency to be packaged individually.
#trap "rm -rf $DEST" EXIT
mkdir -p "${DEST}/src/github.com/liquidgecka/homehub"
( cd "${ROOT}" && git archive HEAD ) | tar -C "${DEST}/src/github.com/liquidgecka/homehub" -xv
cp -rf "${ROOT}/packaging/debian" "${DEST}/debian"
cp -rf "${ROOT}/packaging/debian.conf" "${DEST}/homehub.conf"
cp -rf "${ROOT}/packaging/logrotate.conf" "${DEST}/homehub"
sed -i "s/VERSION/$version/" "${DEST}/debian/rules"
cd "${DEST}/src/github.com/liquidgecka/homehub"
go mod vendor
rm go.mod
tar -C /tmp/build -cp "homehub-${version}" |
    gzip -9 > "/tmp/build/homehub_${version}.orig.tar.gz"

# Build the source package.
debuild -S
debuild

# Backport the package to all supported releases.
cd /tmp/build
for release in $( ubuntu-distro-info --supported ) ; do
    backportpackage $release_flag -w /tmp/build -d $release homehub_${version}-1.dsc
done
