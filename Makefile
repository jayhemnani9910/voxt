# voxt Makefile
# Voice-to-text clipboard tool for Linux/Wayland

# Build configuration
BINARY_NAME := voxt
VERSION := 1.0.0
BUILD_DIR := build
INSTALL_DIR := /usr/local/bin
CONFIG_DIR := $(HOME)/.config/voxt
DESKTOP_DIR := $(HOME)/.local/share/applications
AUTOSTART_DIR := $(HOME)/.config/autostart

# Go configuration
GO := go
GOFLAGS := -v
LDFLAGS := -s -w -X main.Version=$(VERSION)
CGO_ENABLED := 1

# Source files
SRC := $(shell find . -name '*.go' -type f)
ASSETS := $(wildcard assets/*.png assets/*.wav)

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: all build release debug clean install uninstall run deps check assets help desktop autostart

# Default target
all: build

# ============================================================================
# Build Targets
# ============================================================================

## build: Build the application (default)
build: deps assets
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) .
	@echo "$(GREEN)Build complete: ./$(BINARY_NAME)$(NC)"

## release: Build optimized release binary
release: deps assets
	@echo "$(GREEN)Building release $(BINARY_NAME) v$(VERSION)...$(NC)"
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
	@ls -lh $(BINARY_NAME)
	@echo "$(GREEN)Release build complete: ./$(BINARY_NAME)$(NC)"

## debug: Build with debug symbols
debug: deps assets
	@echo "$(YELLOW)Building debug $(BINARY_NAME)...$(NC)"
	$(GO) build $(GOFLAGS) -gcflags="all=-N -l" -o $(BINARY_NAME) .
	@echo "$(YELLOW)Debug build complete: ./$(BINARY_NAME)$(NC)"

## build-dir: Build into build directory
build-dir: deps assets
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "$(GREEN)Built: $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# ============================================================================
# Dependencies
# ============================================================================

## deps: Download and tidy Go dependencies
deps:
	@echo "$(GREEN)Checking dependencies...$(NC)"
	$(GO) mod download
	$(GO) mod tidy

## check: Check system dependencies
check:
	@echo "$(GREEN)Checking system requirements...$(NC)"
	@echo -n "Go: " && $(GO) version || (echo "$(RED)Go not found$(NC)" && exit 1)
	# PortAudio package names vary by distro:
	#   Ubuntu 25.10+: portaudio19-dev
	#   Ubuntu 24.04:  libportaudio-dev
	#   Fedora:        portaudio-devel
	@echo -n "PortAudio: " && (pkg-config --exists portaudio-2.0 && echo "OK") || \
		(echo "$(RED)Missing. Install: sudo apt install portaudio19-dev$(NC)" && exit 1)
	# AppIndicator for system tray:
	#   Ubuntu: libayatana-appindicator3-dev
	#   Fedora: libayatana-appindicator-gtk3-devel
	@echo -n "AppIndicator: " && (pkg-config --exists ayatana-appindicator3-0.1 && echo "OK") || \
		(echo "$(RED)Missing. Install: sudo apt install libayatana-appindicator3-dev$(NC)" && exit 1)
	@echo -n "Input group: " && (groups | grep -q input && echo "OK") || \
		(echo "$(YELLOW)Warning: User not in input group. Run: sudo usermod -a -G input $$USER$(NC)")
	@echo "$(GREEN)All checks passed$(NC)"

# ============================================================================
# Assets
# ============================================================================

## assets: Generate asset files if missing
assets: assets/icon_idle.png assets/icon_recording.png assets/beep_start.wav assets/beep_stop.wav

assets/icon_idle.png assets/icon_recording.png assets/beep_start.wav assets/beep_stop.wav:
	@echo "$(YELLOW)Generating assets...$(NC)"
	@mkdir -p assets
	@python3 -c "\
import struct, zlib, math, os;\
os.makedirs('assets', exist_ok=True);\
def wav(f,freq):\
 sr,d,ns=16000,0.1,1600;\
 ss=[int(math.sin(2*math.pi*freq*i/sr)*((min(i,ns-i,160)/160) if i<160 or i>ns-160 else 1)*16383) for i in range(ns)];\
 open(f,'wb').write(b'RIFF'+struct.pack('<I',3236)+b'WAVEfmt '+struct.pack('<IHHIIHH',16,1,1,sr,sr*2,2,16)+b'data'+struct.pack('<I',3200)+b''.join(struct.pack('<h',s) for s in ss));\
def png(f,r,g,b):\
 sz,rd=22,10.0;\
 d=b'';\
 for y in range(sz):\
  d+=b'\\x00';\
  for x in range(sz):\
   dx,dy=x-11+.5,y-11+.5;dst=math.sqrt(dx*dx+dy*dy);\
   d+=bytes([r,g,b,int(255*(rd-dst)) if rd-1<dst<=rd else 255 if dst<=rd else 0]);\
 c=zlib.compress(d,9);\
 def ch(t,data):crc=zlib.crc32(t+data)&0xffffffff;return struct.pack('>I',len(data))+t+data+struct.pack('>I',crc);\
 open(f,'wb').write(b'\\x89PNG\\r\\n\\x1a\\n'+ch(b'IHDR',struct.pack('>IIBBBBB',sz,sz,8,6,0,0,0))+ch(b'IDAT',c)+ch(b'IEND',b''));\
wav('assets/beep_start.wav',880);wav('assets/beep_stop.wav',440);\
png('assets/icon_idle.png',0,200,0);png('assets/icon_recording.png',200,0,0);\
print('Assets generated')"

## assets-clean: Remove generated assets
assets-clean:
	rm -f assets/*.png assets/*.wav

## assets-regen: Regenerate all assets
assets-regen: assets-clean assets

# ============================================================================
# Install / Uninstall
# ============================================================================

## install: Install binary to system
install: release
	@echo "$(GREEN)Installing $(BINARY_NAME) to $(INSTALL_DIR)...$(NC)"
	@sudo install -Dm755 $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "$(GREEN)Installed: $(INSTALL_DIR)/$(BINARY_NAME)$(NC)"
	@echo ""
	@echo "$(YELLOW)Post-install steps:$(NC)"
	@echo "  1. Add yourself to input group (if not already):"
	@echo "     sudo usermod -a -G input $$USER"
	@echo "  2. Log out and back in"
	@echo "  3. Run: $(BINARY_NAME)"
	@echo "  4. Set your Groq API key in ~/.config/voxt/config.json"

## install-user: Install to user directory (no sudo)
install-user: release
	@echo "$(GREEN)Installing $(BINARY_NAME) to ~/.local/bin...$(NC)"
	@mkdir -p $(HOME)/.local/bin
	@install -m755 $(BINARY_NAME) $(HOME)/.local/bin/$(BINARY_NAME)
	@echo "$(GREEN)Installed: ~/.local/bin/$(BINARY_NAME)$(NC)"
	@echo "$(YELLOW)Make sure ~/.local/bin is in your PATH$(NC)"

## uninstall: Remove installed binary
uninstall:
	@echo "$(RED)Uninstalling $(BINARY_NAME)...$(NC)"
	@sudo rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@rm -f $(HOME)/.local/bin/$(BINARY_NAME)
	@rm -f $(DESKTOP_DIR)/voxt.desktop
	@rm -f $(AUTOSTART_DIR)/voxt.desktop
	@echo "$(GREEN)Uninstalled$(NC)"
	@echo "$(YELLOW)Config remains at: $(CONFIG_DIR)$(NC)"
	@echo "$(YELLOW)Recordings remain at: ~/recordings$(NC)"

## uninstall-all: Remove everything including config
uninstall-all: uninstall
	@echo "$(RED)Removing config and recordings...$(NC)"
	@rm -rf $(CONFIG_DIR)
	@echo "$(YELLOW)Recordings at ~/recordings NOT removed (manual cleanup required)$(NC)"

# ============================================================================
# Desktop Integration
# ============================================================================

## desktop: Create .desktop file for application menu
desktop:
	@echo "$(GREEN)Creating desktop entry...$(NC)"
	@mkdir -p $(DESKTOP_DIR)
	@echo "[Desktop Entry]" > $(DESKTOP_DIR)/voxt.desktop
	@echo "Name=Voxt" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "Comment=Voice-to-text clipboard tool" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "Exec=$(INSTALL_DIR)/$(BINARY_NAME)" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "Icon=audio-input-microphone" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "Terminal=false" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "Type=Application" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "Categories=Utility;Audio;" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "Keywords=voice;speech;transcription;clipboard;" >> $(DESKTOP_DIR)/voxt.desktop
	@echo "$(GREEN)Created: $(DESKTOP_DIR)/voxt.desktop$(NC)"

## autostart: Enable autostart on login
autostart:
	@echo "$(GREEN)Enabling autostart...$(NC)"
	@mkdir -p $(AUTOSTART_DIR)
	@echo "[Desktop Entry]" > $(AUTOSTART_DIR)/voxt.desktop
	@echo "Name=Voxt" >> $(AUTOSTART_DIR)/voxt.desktop
	@echo "Comment=Voice-to-text clipboard tool" >> $(AUTOSTART_DIR)/voxt.desktop
	@echo "Exec=$(INSTALL_DIR)/$(BINARY_NAME)" >> $(AUTOSTART_DIR)/voxt.desktop
	@echo "Icon=audio-input-microphone" >> $(AUTOSTART_DIR)/voxt.desktop
	@echo "Terminal=false" >> $(AUTOSTART_DIR)/voxt.desktop
	@echo "Type=Application" >> $(AUTOSTART_DIR)/voxt.desktop
	@echo "X-GNOME-Autostart-enabled=true" >> $(AUTOSTART_DIR)/voxt.desktop
	@echo "$(GREEN)Autostart enabled: $(AUTOSTART_DIR)/voxt.desktop$(NC)"

## autostart-disable: Disable autostart
autostart-disable:
	@rm -f $(AUTOSTART_DIR)/voxt.desktop
	@echo "$(GREEN)Autostart disabled$(NC)"

# ============================================================================
# Development
# ============================================================================

## run: Build and run
run: build
	@echo "$(GREEN)Running $(BINARY_NAME)...$(NC)"
	./$(BINARY_NAME)

## run-debug: Run with verbose output
run-debug: debug
	@echo "$(YELLOW)Running $(BINARY_NAME) in debug mode...$(NC)"
	./$(BINARY_NAME) 2>&1 | tee voxt.log

## fmt: Format Go code
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GO) fmt ./...

## vet: Run Go vet
vet:
	@echo "$(GREEN)Running vet...$(NC)"
	$(GO) vet ./...

## lint: Run all linters
lint: fmt vet
	@echo "$(GREEN)Linting complete$(NC)"

## test: Run tests (if any)
test:
	@echo "$(GREEN)Running tests...$(NC)"
	$(GO) test -v ./...

# ============================================================================
# Cleanup
# ============================================================================

## clean: Remove build artifacts
clean:
	@echo "$(GREEN)Cleaning...$(NC)"
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f voxt.log
	$(GO) clean

## clean-all: Remove everything including assets
clean-all: clean assets-clean
	rm -rf vendor/

# ============================================================================
# Package (for distribution)
# ============================================================================

## package: Create tarball for distribution
package: release
	@mkdir -p $(BUILD_DIR)/voxt-$(VERSION)
	@cp $(BINARY_NAME) $(BUILD_DIR)/voxt-$(VERSION)/
	@cp README.md $(BUILD_DIR)/voxt-$(VERSION)/ 2>/dev/null || true
	@cp AGENT.md $(BUILD_DIR)/voxt-$(VERSION)/ 2>/dev/null || true
	@echo "#!/bin/bash" > $(BUILD_DIR)/voxt-$(VERSION)/install.sh
	@echo "sudo install -Dm755 voxt /usr/local/bin/voxt" >> $(BUILD_DIR)/voxt-$(VERSION)/install.sh
	@echo "echo 'Installed to /usr/local/bin/voxt'" >> $(BUILD_DIR)/voxt-$(VERSION)/install.sh
	@chmod +x $(BUILD_DIR)/voxt-$(VERSION)/install.sh
	@cd $(BUILD_DIR) && tar -czvf voxt-$(VERSION)-linux-amd64.tar.gz voxt-$(VERSION)
	@echo "$(GREEN)Package created: $(BUILD_DIR)/voxt-$(VERSION)-linux-amd64.tar.gz$(NC)"

## deb: Create .deb package (requires dpkg-deb)
deb: release
	@echo "$(GREEN)Creating .deb package...$(NC)"
	@mkdir -p $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN
	@mkdir -p $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/local/bin
	@mkdir -p $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications
	@cp $(BINARY_NAME) $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/local/bin/
	@echo "Package: voxt" > $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "Version: $(VERSION)" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "Section: utils" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "Priority: optional" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "Architecture: amd64" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "Depends: libportaudio2" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "Maintainer: Local User <local@localhost>" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "Description: Voice-to-text clipboard tool" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo " Press F9 to record, F9 again to stop." >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo " Transcribes audio via Groq API and copies to clipboard." >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/DEBIAN/control
	@echo "[Desktop Entry]" > $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@echo "Name=Voxt" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@echo "Comment=Voice-to-text clipboard tool" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@echo "Exec=/usr/local/bin/voxt" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@echo "Icon=audio-input-microphone" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@echo "Terminal=false" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@echo "Type=Application" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@echo "Categories=Utility;Audio;" >> $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64/usr/share/applications/voxt.desktop
	@dpkg-deb --build $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64
	@mv $(BUILD_DIR)/deb/voxt_$(VERSION)_amd64.deb $(BUILD_DIR)/
	@echo "$(GREEN)Package created: $(BUILD_DIR)/voxt_$(VERSION)_amd64.deb$(NC)"
	@echo "$(YELLOW)Install with: sudo dpkg -i $(BUILD_DIR)/voxt_$(VERSION)_amd64.deb$(NC)"

# ============================================================================
# Help
# ============================================================================

## help: Show this help message
help:
	@echo ""
	@echo "$(GREEN)voxt$(NC) - Voice-to-text clipboard tool"
	@echo ""
	@echo "$(YELLOW)Usage:$(NC)"
	@echo "  make [target]"
	@echo ""
	@echo "$(YELLOW)Build Targets:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | grep -E '(build|release|debug):' | \
		sed -E 's/## /  /' | sed -E 's/: /\t/'
	@echo ""
	@echo "$(YELLOW)Install Targets:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | grep -E '(install|uninstall):' | \
		sed -E 's/## /  /' | sed -E 's/: /\t/'
	@echo ""
	@echo "$(YELLOW)Desktop Integration:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | grep -E '(desktop|autostart):' | \
		sed -E 's/## /  /' | sed -E 's/: /\t/'
	@echo ""
	@echo "$(YELLOW)Development:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | grep -E '(run|fmt|vet|lint|test):' | \
		sed -E 's/## /  /' | sed -E 's/: /\t/'
	@echo ""
	@echo "$(YELLOW)Other:$(NC)"
	@grep -E '^## ' $(MAKEFILE_LIST) | grep -E '(deps|check|clean|assets|package|deb|help):' | \
		sed -E 's/## /  /' | sed -E 's/: /\t/'
	@echo ""
	@echo "$(YELLOW)Quick Start:$(NC)"
	@echo "  make check      # Verify system dependencies"
	@echo "  make build      # Build the binary"
	@echo "  make install    # Install to /usr/local/bin"
	@echo "  make autostart  # Enable autostart on login"
	@echo ""
