package hotkey

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// evdev constants for input event handling
const (
	// Event types
	EV_KEY = 0x01

	// Key states
	KEY_RELEASE = 0
	KEY_PRESS   = 1
	KEY_REPEAT  = 2

	// Max key code (KEY_MAX in Linux)
	KEY_MAX = 0x2ff

	// ioctl constants
	// EVIOCGBIT(ev, len) = _IOC(_IOC_READ, 'E', 0x20 + ev, len)
	// _IOC_READ = 2, 'E' = 0x45
	// For EV_KEY (1): nr = 0x20 + 1 = 0x21
	// Size = 96 bytes (enough for KEY_MAX)
	EVIOCGBIT_EV_KEY = 0x80604521 // _IOC(2, 'E', 0x21, 96)

	// Debounce interval to prevent rapid repeated triggers
	debounceInterval = 300 * time.Millisecond
)

// inputEvent represents a Linux input event structure
type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// HotkeyListener manages global hotkey listening using evdev
type HotkeyListener struct {
	keyCode     uint16
	callback    func()
	devices     []*os.File
	stopCh      chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
	running     bool
	lastPress   time.Time
	lastPressMu sync.Mutex
}

// keyNameToCode maps key names to their evdev codes
var keyNameToCode = map[string]uint16{
	"F1":  59,
	"F2":  60,
	"F3":  61,
	"F4":  62,
	"F5":  63,
	"F6":  64,
	"F7":  65,
	"F8":  66,
	"F9":  67,
	"F10": 68,
	"F11": 87,
	"F12": 88,
}

// NewHotkeyListener creates a new hotkey listener for the specified key
func NewHotkeyListener(keyName string, callback func()) (*HotkeyListener, error) {
	if callback == nil {
		return nil, errors.New("callback cannot be nil")
	}

	keyCode, ok := keyNameToCode[strings.ToUpper(keyName)]
	if !ok {
		return nil, fmt.Errorf("unsupported key: %s (supported keys: F1-F12)", keyName)
	}

	return &HotkeyListener{
		keyCode:  keyCode,
		callback: callback,
		stopCh:   make(chan struct{}),
	}, nil
}

// Start begins listening for the hotkey
func (h *HotkeyListener) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return errors.New("hotkey listener is already running")
	}

	// Find all keyboard input devices
	devices, err := h.findKeyboardDevices()
	if err != nil {
		return fmt.Errorf("failed to find keyboard devices: %w", err)
	}

	if len(devices) == 0 {
		return errors.New("no keyboard devices found - you may need to add your user to the 'input' group")
	}

	h.devices = devices
	h.running = true
	h.stopCh = make(chan struct{})

	// Start listening on each device
	for _, device := range h.devices {
		h.wg.Add(1)
		go h.listenDevice(device)
	}

	return nil
}

// Stop stops listening for the hotkey
func (h *HotkeyListener) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.running = false
	close(h.stopCh)

	// Wait for goroutines to finish FIRST
	h.wg.Wait()

	// Then close all device files
	for _, device := range h.devices {
		device.Close()
	}

	h.devices = nil
}

// findKeyboardDevices finds all keyboard input devices
func (h *HotkeyListener) findKeyboardDevices() ([]*os.File, error) {
	var devices []*os.File
	inputDir := "/dev/input"

	// Check if we can access the input directory
	if _, err := os.Stat(inputDir); os.IsNotExist(err) {
		return nil, errors.New("/dev/input directory not found")
	}

	// Read all event devices
	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /dev/input: %w", err)
	}

	var permissionErrors []string

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "event") {
			continue
		}

		devicePath := filepath.Join(inputDir, entry.Name())

		// Try to open the device
		file, err := os.Open(devicePath)
		if err != nil {
			if os.IsPermission(err) {
				permissionErrors = append(permissionErrors, devicePath)
			}
			// File not opened, no need to close
			continue
		}

		// Check if this device supports keyboard events
		if isKeyboardDevice(file, h.keyCode) {
			devices = append(devices, file)
		} else {
			// Close non-keyboard devices to prevent FD leak
			file.Close()
		}
	}

	// If we found no devices and had permission errors, provide helpful message
	if len(devices) == 0 && len(permissionErrors) > 0 {
		return nil, fmt.Errorf("permission denied accessing input devices. Run: sudo usermod -a -G input $USER && newgrp input")
	}

	return devices, nil
}

// isKeyboardDevice checks if a device supports the specified key
func isKeyboardDevice(file *os.File, keyCode uint16) bool {
	if file == nil {
		return false
	}

	// Buffer to hold key bits (96 bytes = 768 bits, enough for KEY_MAX)
	var keyBits [96]byte

	// SyscallConn, not Fd(): Fd() switches the file to blocking mode, and then
	// the read deadline in listenDevice never fires, so Stop() hung on quit.
	rawConn, err := file.SyscallConn()
	if err != nil {
		return false
	}
	var errno syscall.Errno
	if err := rawConn.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(
			syscall.SYS_IOCTL,
			fd,
			uintptr(EVIOCGBIT_EV_KEY),
			uintptr(unsafe.Pointer(&keyBits[0])),
		)
	}); err != nil || errno != 0 {
		return false
	}

	// Check if the key bit is set
	byteIndex := keyCode / 8
	bitIndex := keyCode % 8

	if int(byteIndex) >= len(keyBits) {
		return false
	}

	return (keyBits[byteIndex] & (1 << bitIndex)) != 0
}

// listenDevice listens for events on a specific device
func (h *HotkeyListener) listenDevice(device *os.File) {
	defer h.wg.Done()

	eventSize := int(unsafe.Sizeof(inputEvent{}))
	buffer := make([]byte, eventSize)

	for {
		select {
		case <-h.stopCh:
			return
		default:
			device.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := device.Read(buffer)
			if err != nil {
				if os.IsTimeout(err) {
					continue
				}
				select {
				case <-h.stopCh:
					return
				default:
					log.Printf("error reading from device %s: %v", device.Name(), err)
					return
				}
			}

			if n != eventSize {
				continue
			}

			// Parse the event
			event := (*inputEvent)(unsafe.Pointer(&buffer[0]))

			// Check if this is our key being pressed
			if event.Type == EV_KEY && event.Code == h.keyCode && event.Value == KEY_PRESS {
				// Debounce: ignore if pressed too recently
				h.lastPressMu.Lock()
				if time.Since(h.lastPress) < debounceInterval {
					h.lastPressMu.Unlock()
					continue
				}
				h.lastPress = time.Now()
				h.lastPressMu.Unlock()

				// Call the callback in a goroutine to avoid blocking
				go func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("hotkey callback panic: %v", r)
						}
					}()
					h.callback()
				}()
			}
		}
	}
}
