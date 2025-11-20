COMPILEDNAME = "git-change-name"
COMPILEDIR = "dist"

HOMEDIR = $(HOME)
USERINSTALLDIR = $(HOMEDIR)/.local/bin

COMPILETARGET = $(COMPILEDIR)/$(COMPILEDNAME)

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
	go build -o $(COMPILETARGET)

.Phony: user-install build
build: $(COMPILETARGET)

# creating local bin in Home dir if it doesn't exist
$(USERINSTALLDIR):
	mkdir -p $(USERINSTALLDIR)

# install for user only
user-install: $(COMPILETARGET) $(USERINSTALLDIR)
	@echo "installing to $(USERINSTALLDIR)"
	cp $(COMPILETARGET) $(USERINSTALLDIR)

# install for system
.Phony: system-install
system-install: $(COMPILETARGET)
	sudo cp $(COMPILETARGET) /usr/bin/

.Phony: system-install-macos
system-install-macos: $(COMPILETARGET)
	sudo cp $(COMPILETARGET) /usr/local/bin/