deb:
	go get -d
	go build
	mkdir -p usr/bin
	cp container usr/bin/vk-container
	fpm -f -s dir -t deb -n "vk-container" -v `cat VERSION` usr/
	rm -rf usr

install: deb
	sudo dpkg -i *.deb

clean:
	rm -rf usr debs/*

.PHONY: setup-deb cont-deb debs clean install
