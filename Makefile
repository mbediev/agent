APP=subutai
CC=go
VERSION=4.0.5-SNAPSHOT
COMMIT=$(shell git rev-parse HEAD)
LDFLAGS=-ldflags "-r /apps/subutai/current/lib -w -s -X main.version=${VERSION} -X main.commit=${COMMIT}"

all:
	$(CC) build ${LDFLAGS} -o $(APP)
install: 
	@mkdir -p $(DESTDIR)/bin
	@cp $(APP) $(DESTDIR)/bin
