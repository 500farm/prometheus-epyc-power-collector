PREFIX=/usr/local
PROGRAM=epyc_power_collector

.PHONY: build clean install uninstall

bin/$(PROGRAM): src/*.go
	go build -o bin/$(PROGRAM) src/*.go

build: bin/$(PROGRAM)

clean:
	@rm -rf ./bin

install: bin/$(PROGRAM) uninstall_program install_program install_systemd

uninstall: uninstall_program uninstall_systemd

install_program:
	mkdir -p $(PREFIX)/bin
	cp bin/$(PROGRAM) $(PREFIX)/bin/

install_systemd:
	cp -n systemd/$(PROGRAM).service /etc/systemd/system/ | true
	systemctl enable $(PROGRAM)
	systemctl start $(PROGRAM)
	systemctl status $(PROGRAM)

uninstall_program:
	systemctl stop $(PROGRAM) 2>/dev/null | true
	systemctl disable $(PROGRAM) 2>/dev/null | true
	rm -f $(PREFIX)/bin/$(PROGRAM) 2>/dev/null | true

uninstall_systemd:
	rm -f /etc/systemd/system/$(PROGRAM).service 2>/dev/null | true
