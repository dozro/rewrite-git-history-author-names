COMPILEDNAME = "git-change-name"
SRC = "cmd/git-change-name/main.go"
COMPILENAMERESIGN = "git-re-sign"
SRCRESIGN = "cmd/git-re-sign/main.go"
COMPILEDIR = "dist"

HOMEDIR = $(HOME)
USERINSTALLDIR = $(HOMEDIR)/.local/bin

COMPILETARGET = $(COMPILEDIR)/$(COMPILEDNAME)
COMPILETARGETRESIGN = $(COMPILEDIR)/$(COMPILENAMERESIGN)

.Phony: precheck
precheck:
	@echo "precheck"
	command -v go >/dev/null 2>&1
	command -v cp >/dev/null 2>&1


# Create target dir
$(COMPILEDIR):
	mkdir $(COMPILEDIR)

# Cleaning
.Phony: clean-ct clean
clean-ct:
	-rm $(COMPILETARGET)
clean: clean-ct
	@echo "cleaning up"
	-rm -d $(COMPILEDIR)

# Compiling
$(COMPILETARGET): precheck
	@echo "compiling"
	go build -o $(COMPILETARGET) $(SRC)

$(COMPILETARGETRESIGN): precheck
	@echo "compiling"
	go build -o $(COMPILETARGETRESIGN) $(SRCRESIGN)

.Phony: user-install build
build: $(COMPILETARGET) $(COMPILETARGETRESIGN)

# creating local bin in Home dir if it doesn't exist
$(USERINSTALLDIR):
	mkdir -p $(USERINSTALLDIR)

# install for user only
user-install: $(COMPILETARGET) $(COMPILETARGETRESIGN) $(USERINSTALLDIR)
	@echo "installing to $(USERINSTALLDIR)"
	@cp $(COMPILETARGET) $(USERINSTALLDIR)
	@cp $(COMPILETARGETRESIGN) $(USERINSTALLDIR)

# install for system
.Phony: system-man-install
system-man-install:
	sudo install -D -m 644 man/git-change-name.1 /usr/share/man/man1/git-change-name.1

.Phony: system-install
system-install: $(COMPILETARGET) $(COMPILETARGETRESIGN) $(COMPILETARGETRESIGN) system-man-install
	sudo cp $(COMPILETARGET) /usr/bin/
	sudo cp $(COMPILETARGETRESIGN) /usr/bin/

.Phony: system-install-macos
system-install-macos: $(COMPILETARGET) $(COMPILETARGETRESIGN)
	sudo cp $(COMPILETARGET) /usr/local/bin/
	sudo cp $(COMPILETARGETRESIGN) /usr/local/bin/