all: build-windows-amd64 build-windows-arm64 build-linux-amd64 build-linux-arm64

build:
	@echo Building ezBadmintonServer for $(GOOS)-$(GOARCH). Version: $(VERSION)
	go build -o ezBadmintonServer-$(GOOS)-$(GOARCH)-$(VERSION)$(EXTENSION)

build-windows-amd64:
	make build GOOS=windows GOARCH=amd64 EXTENSION=.exe

build-windows-arm64:
	make build GOOS=windows GOARCH=arm64 EXTENSION=.exe

build-linux-amd64:
	make build GOOS=linux GOARCH=amd64

build-linux-arm64:
	make build GOOS=linux GOARCH=arm64