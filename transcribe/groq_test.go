package transcribe

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGroq answers both Groq endpoints without touching the network.
type fakeGroq struct {
	whisperText string
	llamaStatus int
	llamaReply  string
	llamaInput  string
	llamaCalls  int
}

func (f *fakeGroq) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == groqAPIEndpoint {
		body, _ := json.Marshal(TranscriptionResponse{Text: f.whisperText})
		return response(http.StatusOK, string(body)), nil
	}

	f.llamaCalls++
	var chat ChatRequest
	if err := json.NewDecoder(req.Body).Decode(&chat); err != nil {
		return nil, err
	}
	f.llamaInput = chat.Messages[0].Content
	var reply ChatResponse
	reply.Choices = make([]struct {
		Message ChatMessage `json:"message"`
	}, 1)
	reply.Choices[0].Message.Content = f.llamaReply
	body, _ := json.Marshal(reply)
	return response(f.llamaStatus, string(body)), nil
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func setup(t *testing.T, fake *fakeGroq) (*Transcriber, string) {
	t.Helper()

	// formatWithLlama builds its own client, which uses the default transport.
	orig := http.DefaultTransport
	http.DefaultTransport = fake
	t.Cleanup(func() { http.DefaultTransport = orig })

	audio := filepath.Join(t.TempDir(), "audio.wav")
	if err := os.WriteFile(audio, []byte("not really audio"), 0600); err != nil {
		t.Fatal(err)
	}

	tr := NewTranscriber("test-key")
	tr.httpClient = &http.Client{Transport: fake}
	return tr, audio
}

func TestTranscribeReturnsLlamaOutput(t *testing.T) {
	fake := &fakeGroq{whisperText: "kem cho", llamaStatus: http.StatusOK, llamaReply: "kem cho\n---\nHow are you"}
	tr, audio := setup(t, fake)

	got, err := tr.Transcribe(audio)
	if err != nil {
		t.Fatal(err)
	}
	if got != fake.llamaReply {
		t.Errorf("got %q, want %q", got, fake.llamaReply)
	}
	if !strings.Contains(fake.llamaInput, "kem cho") {
		t.Errorf("Whisper text was not sent to Llama: %q", fake.llamaInput)
	}
}

func TestTranscribeKeepsWhisperTextWhenLlamaFails(t *testing.T) {
	fake := &fakeGroq{whisperText: "hello", llamaStatus: http.StatusInternalServerError}
	tr, audio := setup(t, fake)

	got, err := tr.Transcribe(audio)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("got %q, want the Whisper text", got)
	}
}

func TestTranscribeKeepsWhisperTextWhenLlamaIsEmpty(t *testing.T) {
	fake := &fakeGroq{whisperText: "hello", llamaStatus: http.StatusOK, llamaReply: "  "}
	tr, audio := setup(t, fake)

	got, err := tr.Transcribe(audio)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("got %q, want the Whisper text", got)
	}
}

func TestTranscribeSkipsLlamaForSilence(t *testing.T) {
	fake := &fakeGroq{whisperText: " "}
	tr, audio := setup(t, fake)

	if _, err := tr.Transcribe(audio); err != nil {
		t.Fatal(err)
	}
	if fake.llamaCalls != 0 {
		t.Errorf("Llama called %d times for empty text", fake.llamaCalls)
	}
}

func TestTranscribeRejectsMissingFile(t *testing.T) {
	tr, audio := setup(t, &fakeGroq{})

	if _, err := tr.Transcribe(audio + ".missing"); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestNewTranscriberNeedsKey(t *testing.T) {
	if NewTranscriber("") != nil {
		t.Error("expected nil without an API key")
	}
}
