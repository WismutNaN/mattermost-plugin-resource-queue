PLUGIN_ID ?= com.scientia.resource-queue
PLUGIN_VERSION ?= 1.0.1
BUNDLE_NAME ?= $(PLUGIN_ID)-$(PLUGIN_VERSION).tar.gz

GO_PLATFORMS = linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64

.PHONY: all build build-server build-webapp dist clean

all: dist

build: build-server build-webapp

build-server:
	@echo "Building server..."
	@mkdir -p server/dist
	$(foreach platform,$(GO_PLATFORMS),\
		$(eval GOOS=$(word 1,$(subst -, ,$(platform)))) \
		$(eval GOARCH=$(word 2,$(subst -, ,$(platform)))) \
		$(eval EXT=$(if $(findstring windows,$(GOOS)),.exe,)) \
		echo "  $(GOOS)/$(GOARCH)..." && \
		cd server && GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o dist/plugin-$(platform)$(EXT) . && cd .. && \
	) true

build-webapp:
	@echo "Building webapp..."
	cd webapp && npm install --legacy-peer-deps && npm run build

dist: build
	@echo "Packing plugin..."
	go run pack.go

clean:
	rm -rf server/dist webapp/dist webapp/node_modules *.tar.gz
