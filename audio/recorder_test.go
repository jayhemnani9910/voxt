package audio

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveWAVHeader(t *testing.T) {
	r := &Recorder{buffer: []int16{1, -1, 2, -2}}
	path := filepath.Join(t.TempDir(), "out.wav")

	if err := r.saveWAV(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 44+8 {
		t.Fatalf("file is %d bytes, want 52", len(data))
	}
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" || string(data[36:40]) != "data" {
		t.Errorf("bad WAV markers: %q", data[:44])
	}
	if got := binary.LittleEndian.Uint32(data[24:28]); got != sampleRate {
		t.Errorf("sample rate = %d, want %d", got, sampleRate)
	}
	if got := binary.LittleEndian.Uint32(data[40:44]); got != 8 {
		t.Errorf("data size = %d, want 8", got)
	}
}
