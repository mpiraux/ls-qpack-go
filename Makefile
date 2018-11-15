.ONESHELL:
all:
	git submodule init
	git submodule update
	cd ls-qpack
	cmake .
	make
	cd ..
	CGO_LDFLAGS_ALLOW=.*ls-qpack.* go build ls-qpack.go
