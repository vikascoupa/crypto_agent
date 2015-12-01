GLIDE = $(GOPATH)/bin/glide

SRCROOT ?= $(realpath .)
BUILD_ROOT ?= $(SRCROOT)

# These are paths used in the docker image
SRCROOT_D = /go/src/crypto_agent
BUILD_ROOT_D = $(SRCROOT_D)/tmp/dist

default: build test

# test runs the unit tests and vets the code
test: format
	go test -timeout=30s -parallel=4
	@$(MAKE) vet

build: format vet
	GO15VENDOREXPERIMENT=1 go build -x \
	-o $(BUILD_ROOT)/crypto_agent \
	-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)" \
	crypto_agent.go

cover: format
	@go tool cover 2>/dev/null; if [ $$? -eq 3 ]; then \
		go get -u golang.org/x/tools/cmd/cover; \
	fi
	@go test $(TEST) -cover -test.v=true -test.coverprofile=c.out
	sed -i -e "s#.*/\(.*\.go\)#\./\\1#" c.out
	@go tool cover -html=c.out -o c.html

trace: format
	@go test $(TEST) -trace trace.out crypto_agent
	@go tool trace crypto_agent.test trace.out

# vet runs the Go source code static analysis tool `vet` to find
# any common errors.
vet:
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@echo "go tool vet *.go"
	@go tool vet *.go ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for reviewal."; \
	fi

# source files.
format:
	go fmt
	go get github.com/Masterminds/glide
	if [ ! -d vendor ]; then $(GLIDE) install --import; fi

dist:
	docker run --rm \
	           -v $(SRCROOT):$(SRCROOT_D) \
	           -w $(SRCROOT_D) \
	           -e BUILD_ROOT=$(BUILD_ROOT_D) \
						 -e UID=`id -u` \
						 -e GID=`id -g` \
	           golang \
	           make distbuild

distbuild: build
	chown -R $(UID):$(GID) $(SRCROOT)

.PHONY: bin default format test updatedep restoredep vet dist
