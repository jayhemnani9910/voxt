package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

func main() {
	// Create assets directory if it doesn't exist
	assetsDir := filepath.Join(".", "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		log.Fatalf("Failed to create assets directory: %v", err)
	}

	// Generate PNG icons
	if err := generateIcon(filepath.Join(assetsDir, "icon_idle.png"), color.RGBA{0, 255, 0, 255}); err != nil {
		log.Fatalf("Failed to generate icon_idle.png: %v", err)
	}
	log.Println("Generated icon_idle.png")

	if err := generateIcon(filepath.Join(assetsDir, "icon_recording.png"), color.RGBA{255, 0, 0, 255}); err != nil {
		log.Fatalf("Failed to generate icon_recording.png: %v", err)
	}
	log.Println("Generated icon_recording.png")

	// Generate WAV beeps
	if err := generateBeep(filepath.Join(assetsDir, "beep_start.wav"), 880); err != nil {
		log.Fatalf("Failed to generate beep_start.wav: %v", err)
	}
	log.Println("Generated beep_start.wav")

	if err := generateBeep(filepath.Join(assetsDir, "beep_stop.wav"), 440); err != nil {
		log.Fatalf("Failed to generate beep_stop.wav: %v", err)
	}
	log.Println("Generated beep_stop.wav")

	log.Println("All assets generated successfully!")
}

// generateIcon creates a 22x22 PNG with a solid colored circle
func generateIcon(filename string, col color.RGBA) error {
	const size = 22
	const radius = 10.0

	// Create a transparent image
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Draw a filled circle
	center := size / 2.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - center + 0.5
			dy := float64(y) - center + 0.5
			distance := math.Sqrt(dx*dx + dy*dy)

			if distance <= radius {
				// Apply simple antialiasing at the edge
				if distance > radius-1 {
					alpha := uint8(255 * (radius - distance))
					img.Set(x, y, color.RGBA{col.R, col.G, col.B, alpha})
				} else {
					img.Set(x, y, col)
				}
			}
		}
	}

	// Save to file
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// generateBeep creates a WAV file with a sine wave at the specified frequency
func generateBeep(filename string, frequency float64) error {
	const (
		sampleRate = 16000 // Hz
		duration   = 0.1   // seconds
		bitDepth   = 16    // bits
		channels   = 1     // mono
	)

	numSamples := int(float64(sampleRate) * duration)
	samples := make([]int16, numSamples)

	// Generate sine wave samples
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		value := math.Sin(2 * math.Pi * frequency * t)

		// Apply a simple envelope to avoid clicks (fade in/out)
		envelope := 1.0
		fadeLength := numSamples / 10 // 10% fade
		if i < fadeLength {
			envelope = float64(i) / float64(fadeLength)
		} else if i > numSamples-fadeLength {
			envelope = float64(numSamples-i) / float64(fadeLength)
		}

		samples[i] = int16(value * envelope * 32767.0 * 0.5) // 50% volume
	}

	// Create WAV file
	var buf bytes.Buffer

	// RIFF header
	buf.WriteString("RIFF")
	dataSize := numSamples * channels * (bitDepth / 8)
	fileSize := 36 + dataSize
	binary.Write(&buf, binary.LittleEndian, uint32(fileSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))         // chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))          // PCM format
	binary.Write(&buf, binary.LittleEndian, uint16(channels))   // channels
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate)) // sample rate
	byteRate := sampleRate * channels * (bitDepth / 8)
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate)) // byte rate
	blockAlign := channels * (bitDepth / 8)
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign)) // block align
	binary.Write(&buf, binary.LittleEndian, uint16(bitDepth))   // bits per sample

	// data chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))
	for _, sample := range samples {
		binary.Write(&buf, binary.LittleEndian, sample)
	}

	// Write to file
	return os.WriteFile(filename, buf.Bytes(), 0644)
}
