package audio

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gordonklaus/portaudio"
)

const (
	sampleRate    = 16000 // 16kHz for Groq API
	channels      = 1     // Mono
	bitsPerSample = 16    // 16-bit
	frameSize     = 512   // Number of frames per buffer

	// Max recording duration: 10 minutes (prevents OOM)
	maxBufferSize = sampleRate * 60 * 10 // ~19MB max
)

// Global PortAudio initialization (singleton pattern)
var (
	paInitOnce sync.Once
	paTermOnce sync.Once
	paInitErr  error
)

// Recorder handles audio recording using PortAudio
type Recorder struct {
	recordingsDir string
	stream        *portaudio.Stream
	recording     bool
	buffer        []int16
	done          chan struct{}
	recordingMu   sync.Mutex // protects recording flag
	bufferMu      sync.Mutex // protects buffer
	stopChan      chan struct{}
	closeOnce     sync.Once  // prevents double-close panic on stopChan
	ErrChan       chan error // reports recording errors to caller
}

// initPortAudio initializes PortAudio exactly once
func initPortAudio() error {
	paInitOnce.Do(func() {
		paInitErr = portaudio.Initialize()
	})
	return paInitErr
}

// NewRecorder creates a new audio recorder
// recordingsDir: base directory where recordings will be saved (e.g., "~/recordings")
func NewRecorder(recordingsDir string) (*Recorder, error) {
	// Expand ~ to home directory
	recordingsDir = expandHome(recordingsDir)

	// Initialize PortAudio (singleton - safe to call multiple times)
	if err := initPortAudio(); err != nil {
		return nil, fmt.Errorf("failed to initialize portaudio: %w", err)
	}

	return &Recorder{
		recordingsDir: recordingsDir,
		buffer:        make([]int16, 0),
		done:          make(chan struct{}),
		ErrChan:       make(chan error, 1), // buffered to prevent blocking
	}, nil
}

// expandHome expands ~ to the user's home directory
func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if len(path) == 1 {
		return home
	}
	if path[1] == '/' {
		return filepath.Join(home, path[2:])
	}
	return path
}

// Start begins recording audio
func (r *Recorder) Start() error {
	r.recordingMu.Lock()
	if r.recording {
		r.recordingMu.Unlock()
		return fmt.Errorf("already recording")
	}
	r.recording = true
	r.recordingMu.Unlock()

	// Reset buffer for new recording
	r.bufferMu.Lock()
	r.buffer = make([]int16, 0)
	r.bufferMu.Unlock()

	r.done = make(chan struct{})
	r.stopChan = make(chan struct{})
	r.closeOnce = sync.Once{} // reset for new recording

	// Open default input device
	inputBuffer := make([]int16, frameSize)

	stream, err := portaudio.OpenDefaultStream(
		channels, // input channels
		0,        // output channels (we're only recording)
		float64(sampleRate),
		frameSize,
		inputBuffer,
	)
	if err != nil {
		r.recordingMu.Lock()
		r.recording = false
		r.recordingMu.Unlock()
		return fmt.Errorf("failed to open audio stream: %w", err)
	}

	// Start the stream
	if err := stream.Start(); err != nil {
		stream.Close()
		r.recordingMu.Lock()
		r.recording = false
		r.recordingMu.Unlock()
		return fmt.Errorf("failed to start audio stream: %w", err)
	}

	// Only assign r.stream AFTER successful stream.Start()
	r.stream = stream

	// Start recording in a goroutine
	go r.record(inputBuffer)

	return nil
}

// record continuously reads from the audio stream
func (r *Recorder) record(inputBuffer []int16) {
	defer close(r.done)
	for {
		select {
		case <-r.stopChan:
			return
		default:
			if err := r.stream.Read(); err != nil {
				// Report error to caller instead of silent exit
				select {
				case r.ErrChan <- fmt.Errorf("audio device error: %w", err):
				default:
					// Channel full, log instead
					log.Printf("Audio device error: %v", err)
				}
				return
			}
			r.bufferMu.Lock()
			// Check buffer size limit to prevent OOM
			if len(r.buffer) >= maxBufferSize {
				r.bufferMu.Unlock()
				select {
				case r.ErrChan <- fmt.Errorf("max recording duration reached (10 minutes)"):
				default:
				}
				return
			}
			r.buffer = append(r.buffer, inputBuffer...)
			r.bufferMu.Unlock()
		}
	}
}

// Stop stops recording and saves the audio to a WAV file
// Returns the path to the saved file
func (r *Recorder) Stop() (string, error) {
	r.recordingMu.Lock()
	if !r.recording {
		r.recordingMu.Unlock()
		return "", fmt.Errorf("not currently recording")
	}
	r.recording = false
	r.recordingMu.Unlock()

	// Signal goroutine to stop (use sync.Once to prevent double-close panic)
	r.closeOnce.Do(func() {
		close(r.stopChan)
	})

	// Wait for recording goroutine to finish with timeout
	select {
	case <-r.done:
		// Normal completion
	case <-time.After(5 * time.Second):
		log.Println("Warning: recording goroutine timed out")
	}

	// Stop and close stream with nil checks
	var stopErr error
	if r.stream != nil {
		if err := r.stream.Stop(); err != nil {
			stopErr = fmt.Errorf("failed to stop audio stream: %w", err)
		}

		if err := r.stream.Close(); err != nil {
			if stopErr != nil {
				return "", fmt.Errorf("%v; failed to close audio stream: %w", stopErr, err)
			}
			return "", fmt.Errorf("failed to close audio stream: %w", err)
		}
		r.stream = nil // clear reference
	}

	if stopErr != nil {
		return "", stopErr
	}

	// Check for empty buffer before saving WAV
	r.bufferMu.Lock()
	bufferLen := len(r.buffer)
	r.bufferMu.Unlock()

	if bufferLen == 0 {
		return "", fmt.Errorf("no audio data recorded")
	}

	// Check for WAV format overflow (uint32 limit)
	if int64(bufferLen)*2 > math.MaxUint32-36 {
		return "", fmt.Errorf("recording too large for WAV format")
	}

	// Generate file path: ~/recordings/YYYY-MM-DD/audio_HHMMSS.wav
	now := time.Now()
	dateDir := filepath.Join(r.recordingsDir, now.Format("2006-01-02"))

	// Create date directory if it doesn't exist
	// Owner-only: recordings are the user's voice.
	if err := os.MkdirAll(dateDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create recordings directory: %w", err)
	}

	filename := fmt.Sprintf("audio_%s.wav", now.Format("150405"))
	filePath := filepath.Join(dateDir, filename)

	// Save the recording as WAV file
	if err := r.saveWAV(filePath); err != nil {
		// Clean up partial file on error
		os.Remove(filePath)
		return "", fmt.Errorf("failed to save WAV file: %w", err)
	}

	return filePath, nil
}

// saveWAV writes the recorded audio data to a WAV file with proper RIFF header
func (r *Recorder) saveWAV(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	r.bufferMu.Lock()
	defer r.bufferMu.Unlock()

	// Calculate sizes
	dataSize := uint32(len(r.buffer) * 2) // 2 bytes per sample (16-bit)
	fileSize := dataSize + 36             // 36 bytes for WAV header

	// Write RIFF header
	if _, err := file.WriteString("RIFF"); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, fileSize); err != nil {
		return err
	}
	if _, err := file.WriteString("WAVE"); err != nil {
		return err
	}

	// Write fmt chunk
	if _, err := file.WriteString("fmt "); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(16)); err != nil { // fmt chunk size
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(1)); err != nil { // audio format (1 = PCM)
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(channels)); err != nil { // number of channels
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(sampleRate)); err != nil { // sample rate
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint32(sampleRate*channels*bitsPerSample/8)); err != nil { // byte rate
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(channels*bitsPerSample/8)); err != nil { // block align
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, uint16(bitsPerSample)); err != nil { // bits per sample
		return err
	}

	// Write data chunk
	if _, err := file.WriteString("data"); err != nil {
		return err
	}
	if err := binary.Write(file, binary.LittleEndian, dataSize); err != nil {
		return err
	}

	// Write audio data
	for _, sample := range r.buffer {
		if err := binary.Write(file, binary.LittleEndian, sample); err != nil {
			return err
		}
	}

	return nil
}

// Close cleans up resources
func (r *Recorder) Close() error {
	r.recordingMu.Lock()
	isRecording := r.recording
	r.recordingMu.Unlock()

	if isRecording {
		r.Stop()
	}

	// Terminate PortAudio (singleton - safe to call multiple times)
	paTermOnce.Do(func() {
		portaudio.Terminate()
	})
	return nil
}

// IsRecording returns whether the recorder is currently recording
func (r *Recorder) IsRecording() bool {
	r.recordingMu.Lock()
	defer r.recordingMu.Unlock()
	return r.recording
}
