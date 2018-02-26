GOOS       ?= $(shell go env GOOS)
GOARCH     ?= $(shell go env GOARCH)
export XC_OS = $(GOOS)
export XC_ARCH = $(GOARCH)

SUFFIX     := $(GOOS)_$(GOARCH)

# When the tag name is not available, use the commit hash
TRAVIS_TAG ?= $(shell git rev-parse --short HEAD)

CMD        := $(notdir $(wildcard cmd/*))
ARCHIVE    := $(addsuffix _$(TRAVIS_TAG)_$(SUFFIX).tgz,$(CMD))

GO_PKGS    := \
	github.com/golang/lint/golint \
	github.com/mitchellh/gox


default: test

test:
	go install ./...
	go test -v ./...
	go vet -x ./...
	${GOPATH}/bin/golint -set_exit_status ./...

archive: $(ARCHIVE)

bin: $(patsubst %,pkg/%_$(SUFFIX),$(CMD))

pkg/%_$(SUFFIX): cmd/%
	CGO_ENABLED=0 ./scripts/build.sh $*

%_$(TRAVIS_TAG)_$(SUFFIX).tgz: pkg/%_$(SUFFIX)
	cp cmd/$*/*.toml cmd/$*/USAGE.md LICENSE $<
	tar -c -z -C pkg/ -f $@ $(notdir $<)

clean:
	rm -rf pkg/ *.tgz

bootstrap:
	go get -u ./...
	go get $(GO_PKGS)

.PHONY: test archive bin clean bootstrap
