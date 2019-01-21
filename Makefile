.ONESHELL:
all:
	git submodule init
	git submodule update
	cd ls-qpack
	cmake .
	make ls-qpack
	cd ..
	CGO_LDFLAGS_ALLOW=.*ls-qpack.* go build ls-qpack.go
