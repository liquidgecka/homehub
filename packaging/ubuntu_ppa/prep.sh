#!/bin/bash -e

check_binary() {
    if [ -z "$(which "$1")" ] ; then
        echo "$1 is a required tool."
        exit 1
    fi
}

usage() {
  echo "$0: Generate debian packages."
  echo ""
  echo "Arguments:"
  echo "  -h --help   Show this message."
  echo "  --version   Set the version to build."
}

makesource() {
  # Make sure that the .git directory exists.
  if [ ! -d .git ] ; then
    echo "makesource must be run from the top of the repo." >&2
    exit 1
  fi

  # Make sure that all the expected binaries are installed.
  check_binary git
  check_binary gzip
  check_binary go
  check_binary tar

  # Get the current git commit.
  current_commit="$(git rev-parse HEAD)"

  # Refuse to build if the branch is not clean so that we don't end up
  # with inconsistent builds.
  if [ -n "$(git status --short)" ] ; then
    echo "The repository has uncomitted changes. Building from this" >&2
    echo "branch will produce a build that is not consistent." >&2
    exit 1
  fi

  # Rather than maintaining the changelog file manually like most debian
  # packages we generate one from the commit log in git.
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
        echo "homehub (${this_version}-__DISTRO__1) __DISTRO__; urgency=low"
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
done > "/tmp/build/changelog"
if [ "${dirty}" == "y" ] ; then
    echo "${dirty} are still outstanding"
    echo "Repo is dirty. Can not build."
    exit 1
fi

set -x

# Setup some variables for use during the build.
echo "${version}" > /tmp/build/version
DEST="/tmp/build/homehub-${version}"
export GOPATH="$DEST"

# Setup the build environment. Since most deb building services do not allow
# downloads we need to convert the modules into vendorized directories. This
# will allow the source to act as a single package rather than requiring
# every single dependency to be packaged individually.
mkdir -p "${DEST}/src/github.com/liquidgecka"
cp -r "${ROOT}" "${DEST}/src/github.com/liquidgecka/homehub"
rm -rf "${DEST}/src/github.com/liquidgecka/homehub/packaging"
cp -rf "${ROOT}/packaging/debian.conf" "${DEST}/homehub.conf"
cp -rf "${ROOT}/packaging/logrotate.conf" "${DEST}/homehub"
(
    cd "${DEST}/src/github.com/liquidgecka/homehub" &&
    go mod vendor &&
    rm go.mod
)
  tar  \
      --create \
      --directory "/tmp/build" \
      --exclude "homehub-${version}/debian" \
      --exclude "homehub-${version}/.git" \
      --exclude "homehub-${version}/.github" \
      --exclude "homehub-${version}/.gitignore" \
      --exclude "homehub-${version}/packaging" \
      --exclude "homehub-${version}/src/github.com/liquidgecka/homehub/packaging" \
      --exclude-vcs \
      --group 0 \
      --mtime="$(date +"%Y-%m-%d 00:00:00")" \
      --owner 0 \
      --preserve-permissions \
      --sort=name \
      "homehub-${version}" |
  gzip -n -9 > "/tmp/build/homehub_${version}.orig.tar.gz"
  md5sum "/tmp/build/homehub_${version}.orig.tar.gz"
}

while true ; do
  case "$1" in
    -h|--help)
      usage
      ;;
    --version)
      version="${2}"
      shift
      ;;
    --)
      shift
      break
      ;;
    "")
      break
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
  shift
done

# Check and see what action is being performed via the command line.
case "$1" in
  origtar)
    ;;
  release)
    ;;
esac

check_binary git
check_binary gzip
check_binary go
check_binary tar

# Calculate the root of the repo.
ROOT="$( cd "$(dirname $0)/../.." && pwd )"

