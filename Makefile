# This is how we want to name the binary output
OUTPUT=scraper
# These are the values we want to pass for Version and BuildTime
GITTAG=`git describe --tags`
BUILD_TIME=`date +%FT%T%z`
# Setup the -ldflags option for go build here, interpolate the variable values
ifdef withversion
	LDFLAGS=-ldflags "-X main.GitTag=${GITTAG} -X main.BuildTime=${BUILD_TIME}"
else
	LDFLAGS=
endif

all:
	go build ${LDFLAGS} -o ${OUTPUT} main.go cmdstore.go

