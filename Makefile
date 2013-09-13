deb:
	go build
	mkdir -p usr/bin
	cp container usr/bin/ar-container
	bundle exec fpm -f -s dir -t deb -n "ar-container" -v `cat VERSION` usr/
	rm -rf usr

install: deb
	sudo dpkg -i *.deb

clean:
	rm -rf usr debs/*

.PHONY: setup-deb cont-deb debs clean install
