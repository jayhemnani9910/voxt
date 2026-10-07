package main

import (
	"context"
	"embed"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"voxt/audio"
	"voxt/clipboard"
	"voxt/config"
	"voxt/db"
	"voxt/hotkey"
	"voxt/notify"
	"voxt/transcribe"
	"voxt/tray"
)

//go:embed assets/beep_start.wav assets/beep_stop.wav
var soundFS embed.FS

//go:embed assets/icon_idle.png assets/icon_recording.png
var iconFS embed.FS

// AppState represents the current state of the application
type AppState int

const (
	StateIdle AppState = iota
	StateRecording
	StateTranscribing
)

type App struct {
	// UI components
	tray     *tray.Tray
	notifier *notify.Notifier

	// Core components
	config         *config.Config
	recorder       *audio.Recorder
	transcriber    *transcribe.Transcriber
	database       *db.DB
	hotkeyListener *hotkey.HotkeyListener

	// State
	state      AppState
	mu         sync.Mutex
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	app := &App{
		state: StateIdle,
	}

	if err := app.init(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Run the tray (blocking)
	app.tray.Run(func() {
		log.Println("voxt started - press", app.config.Hotkey, "to toggle recording")
	})

	app.cleanup()
}

func (a *App) init() error {
	// Initialize context
	a.ctx, a.cancelFunc = context.WithCancel(context.Background())

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Config load warning: %v", err)
		cfg = config.DefaultConfig()
	}
	a.config = cfg

	// Initialize database
	database, err := db.Init()
	if err != nil {
		log.Printf("Database init warning: %v", err)
	}
	a.database = database

	// Check API key
	if cfg.GroqAPIKey == "" {
		log.Println("No API key configured - set it in settings")
	}

	// Initialize clipboard
	if err := clipboard.Init(); err != nil {
		log.Printf("Clipboard init warning: %v", err)
	}

	// Initialize notifier (DBus)
	notifier, err := notify.NewNotifier()
	if err != nil {
		log.Printf("Notifier init warning: %v", err)
	} else {
		a.notifier = notifier
	}

	// Initialize transcriber
	a.transcriber = transcribe.NewTranscriber(cfg.GroqAPIKey)

	// Initialize recorder
	recorder, err := audio.NewRecorder(cfg.RecordingsDir)
	if err != nil {
		return err
	}
	a.recorder = recorder

	// Start goroutine to monitor recorder errors
	go a.monitorRecorderErrors()

	// Load tray icons
	idleIcon, err := iconFS.ReadFile("assets/icon_idle.png")
	if err != nil {
		return err
	}
	recordingIcon, err := iconFS.ReadFile("assets/icon_recording.png")
	if err != nil {
		return err
	}

	// Create tray
	a.tray = tray.NewTray(idleIcon, recordingIcon)
	a.tray.SetOnSettings(a.showSettings)

	// Initialize hotkey listener
	hk, err := hotkey.NewHotkeyListener(cfg.Hotkey, func() {
		a.toggleRecording()
	})
	if err != nil {
		log.Printf("Hotkey init warning: %v", err)
	} else {
		a.hotkeyListener = hk
		go func() {
			if err := hk.Start(); err != nil {
				log.Printf("Hotkey listener error: %v", err)
			}
		}()
	}

	return nil
}

func (a *App) monitorRecorderErrors() {
	if a.recorder == nil {
		return
	}
	for {
		select {
		case <-a.ctx.Done():
			return
		case err := <-a.recorder.ErrChan:
			if err != nil {
				log.Printf("Recorder error: %v", err)
				if a.notifier != nil {
					a.notifier.ShowError("Recording error: " + err.Error())
				}
				// The recorder goroutine has exited but the recorder still counts
				// as recording; Stop clears that (or every later Start fails) and
				// saves what was captured, e.g. at the 10 minute cap.
				a.mu.Lock()
				if a.state == StateRecording {
					a.tray.SetIdle()
					if audioPath, err := a.recorder.Stop(); err == nil {
						a.state = StateTranscribing
						go a.transcribeAndNotify(audioPath)
					} else {
						log.Printf("Recording stop after error failed: %v", err)
						a.state = StateIdle
					}
				}
				a.mu.Unlock()
			}
		}
	}
}

func (a *App) toggleRecording() {
	a.mu.Lock()
	defer a.mu.Unlock()

	switch a.state {
	case StateIdle:
		a.startRecording()
	case StateRecording:
		a.stopRecording()
	case StateTranscribing:
		// Ignore - already processing
	}
}

func (a *App) startRecording() {
	if a.recorder == nil {
		log.Println("Recorder not available")
		if a.notifier != nil {
			a.notifier.ShowError("Microphone not available")
		}
		return
	}

	log.Println("Starting recording...")

	// Update tray icon
	a.tray.SetRecording()

	// Play start beep
	a.playBeep("start")

	// Set state before starting recording
	a.state = StateRecording

	// Start recording
	if err := a.recorder.Start(); err != nil {
		log.Printf("Recording start failed: %v", err)
		a.state = StateIdle
		a.tray.SetIdle()
		if a.notifier != nil {
			a.notifier.ShowError("Failed to start recording")
		}
		return
	}
}

func (a *App) stopRecording() {
	if a.recorder == nil {
		return
	}

	log.Println("Stopping recording...")

	// Stop recording before the beep, so the beep is not in the audio
	audioPath, err := a.recorder.Stop()
	a.playBeep("stop")
	if err != nil {
		log.Printf("Recording stop failed: %v", err)
		a.tray.SetIdle()
		a.state = StateIdle
		if a.notifier != nil {
			a.notifier.ShowError("Failed to save recording")
		}
		return
	}

	log.Printf("Audio saved: %s", audioPath)

	// Set transcribing state (tray stays idle-colored during transcription)
	a.state = StateTranscribing
	a.tray.SetIdle()

	// Transcribe in background
	go a.transcribeAndNotify(audioPath)
}

func (a *App) transcribeAndNotify(audioPath string) {
	defer func() {
		a.mu.Lock()
		a.state = StateIdle
		a.mu.Unlock()
	}()

	// Check context before starting
	if a.ctx.Err() != nil {
		log.Println("Transcription cancelled: context done")
		return
	}

	// The key may have been added in Settings since startup; pick it up.
	if a.config.GroqAPIKey == "" || a.transcriber == nil {
		if cfg, err := config.Load(); err == nil && cfg.GroqAPIKey != "" {
			a.config.GroqAPIKey = cfg.GroqAPIKey
			a.transcriber = transcribe.NewTranscriber(cfg.GroqAPIKey)
		}
	}

	if a.config.GroqAPIKey == "" {
		log.Println("No API key configured")
		if a.notifier != nil {
			a.notifier.ShowError("No API key configured. Set it in Settings.")
		}
		return
	}

	if a.transcriber == nil {
		log.Println("Transcriber not initialized - API key may be empty")
		if a.notifier != nil {
			a.notifier.ShowError("API key not configured")
		}
		return
	}

	// Transcribe with context for cancellation support
	text, err := a.transcriber.TranscribeWithContext(a.ctx, audioPath)
	if err != nil {
		log.Printf("Transcription failed: %v", err)
		if a.notifier != nil {
			a.notifier.ShowError("Transcription failed")
		}
		return
	}

	// Check context after transcription
	if a.ctx.Err() != nil {
		log.Println("Transcription cancelled: context done")
		return
	}

	// Save to database
	if a.database != nil {
		if _, err := a.database.Save(text, audioPath, 0); err != nil {
			log.Printf("Failed to save to database: %v", err)
		}
	}

	// Copy to clipboard immediately
	if err := clipboard.Copy(text); err != nil {
		log.Printf("Clipboard copy failed: %v", err)
	} else {
		log.Println("Text copied to clipboard")
	}

	// Show notification
	if a.notifier != nil {
		if err := a.notifier.ShowTranscription(text); err != nil {
			log.Printf("Failed to show notification: %v", err)
		}
	}
}

func (a *App) playBeep(beepType string) {
	var filename string
	if beepType == "start" {
		filename = "assets/beep_start.wav"
	} else {
		filename = "assets/beep_stop.wav"
	}

	data, err := soundFS.ReadFile(filename)
	if err != nil {
		log.Printf("Could not load beep sound: %v", err)
		return
	}

	// Write to temp file and play with paplay
	tmpFile, err := os.CreateTemp("", "voxt_beep_*.wav")
	if err != nil {
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return
	}

	// Close file before playing
	tmpFile.Close()

	// Try paplay first (PulseAudio/PipeWire), then aplay (ALSA)
	// Use timeout context to prevent zombie processes
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := exec.CommandContext(ctx, "paplay", tmpPath).Run(); err != nil {
		// Try aplay as fallback
		ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel2()
		exec.CommandContext(ctx2, "aplay", "-q", tmpPath).Run()
	}
}

func (a *App) showSettings() {
	// For now, open config file with xdg-open
	// TODO: Implement GTK4 settings panel in a separate process
	configPath, err := config.GetConfigPath()
	if err != nil {
		log.Printf("Failed to get config path: %v", err)
		return
	}
	log.Printf("Opening settings: %s", configPath)
	cmd := exec.Command("xdg-open", configPath)
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open settings: %v", err)
		return
	}
	go cmd.Wait() // reap the child
}

func (a *App) cleanup() {
	log.Println("Cleaning up...")

	// Cancel context first
	if a.cancelFunc != nil {
		a.cancelFunc()
	}

	if a.hotkeyListener != nil {
		a.hotkeyListener.Stop()
	}
	if a.recorder != nil {
		a.recorder.Close()
	}
	if a.database != nil {
		a.database.Close()
	}
	if a.notifier != nil {
		a.notifier.Close()
	}
}
