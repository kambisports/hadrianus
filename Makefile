export GOOS=linux
export GOARCH=amd64

build: hadrianus

all: clean build
all-mac: clean build-mac

hadrianus: main.go
	go build

build-mac: export GOOS=darwin
build-mac: hadrianus

clean:
	rm -f hadrianus
