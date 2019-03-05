GOOS := $(shell echo $${GOOS:-$$(go env GOOS)})
GOARCH := $(shell echo $${GOARCH:-$$(go env GOARCH)})

all: blackbox blackbox-linux

blackbox: *.go
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go test -c -o blackbox-$(GOOS)-$(GOARCH)

blackbox-linux:
	GOOS=linux $(MAKE) blackbox
