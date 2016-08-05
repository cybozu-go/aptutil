#!/bin/sh -e

usage() {
    echo "Usage: build.sh [go-apt-cacher|go-apt-mirror]"
    echo
    exit 2
}


# sanity check -------

if [ $# -ne 1 ]; then
    usage
fi

if [ "$1" != "go-apt-cacher" -a "$1" != "go-apt-mirror" ]; then
    usage
fi

TARGET="$1"
XC_OS="${XC_OS:-$(go env GOOS)}"
XC_ARCH="${XC_ARCH:-$(go env GOARCH)}"

# build ------

echo "Building..."

${GOPATH}/bin/gox \
    -os="${XC_OS}" \
    -arch="${XC_ARCH}" \
    -output "pkg/${TARGET}_{{.OS}}_{{.Arch}}/${TARGET}" \
    ./cmd/${TARGET}
