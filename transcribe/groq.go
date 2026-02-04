package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	groqAPIEndpoint  = "https://api.groq.com/openai/v1/audio/transcriptions"
	llamaAPIEndpoint = "https://api.groq.com/openai/v1/chat/completions"
	whisperModel     = "whisper-large-v3"
	llamaModel       = "llama-3.1-8b-instant"

	// No prompt for Whisper - let it auto-detect language naturally

	// Prompt for Llama to transliterate and format transcription
	formatPrompt = `Process this transcription:

1. If the text is normal English sentences: return UNCHANGED. Do not modify.

2. If it contains Devanagari script (Hindi: मैं), Gujarati script (કેમ), or other non-Latin characters:
   - Transliterate to romanized Latin letters (how it sounds)
   - Add English translation after "---"

3. If it's clearly romanized Hindi/Gujarati (words like "kem", "che", "hoon", "hai", "kya", "bhai", "aap"):
   - Keep the romanized text
   - Add English translation after "---"

IMPORTANT: Common English words should stay as English. Only transliterate actual non-English.

Examples:
- Input: "I am fine" → Output: "I am fine"
- Input: "Hello how are you" → Output: "Hello how are you"
- Input: "मैं ठीक हूं" → Output: "main theek hoon\n---\nI am fine"
- Input: "kem cho bhai" → Output: "kem cho bhai\n---\nHow are you brother"
- Input: "The meeting is at 3pm" → Output: "The meeting is at 3pm"

Only output the processed text, nothing else.

Text:
%s`

	// Size limits
	maxAudioFileSize = 25 * 1024 * 1024 // 25MB
	minAudioFileSize = 1
	maxResponseSize  = 10 * 1024 * 1024 // 10MB

	// Retry configuration
	maxRetries        = 3
	initialRetryDelay = 1 * time.Second
	maxRetryDelay     = 10 * time.Second
	defaultTimeout    = 300 * time.Second // 5 minutes
	llamaTimeout      = 30 * time.Second  // Llama is fast
)

// Transcriber handles audio transcription using Groq's Whisper API
type Transcriber struct {
	apiKey     string
	httpClient *http.Client
}

// TranscriptionResponse represents the JSON response from Groq API
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// ErrorResponse represents an error response from Groq API
type ErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ChatMessage represents a message in chat completion
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// NewTranscriber creates a new Transcriber instance with the provided API key
func NewTranscriber(apiKey string) *Transcriber {
	if apiKey == "" {
		return nil
	}
	return &Transcriber{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// doWithRetry performs an HTTP request with exponential backoff retry and rate limit handling
func (t *Transcriber) doWithRetry(ctx context.Context, req *http.Request, client *http.Client) (*http.Response, error) {
	var lastErr error
	delay := initialRetryDelay

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay = min(delay*2, maxRetryDelay)
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Handle rate limiting (429) and service unavailable (503)
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			resp.Body.Close()
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil {
					delay = time.Duration(seconds) * time.Second
				}
			}
			lastErr = fmt.Errorf("rate limited (status %d)", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// formatWithLlama sends text to Llama to add separator between romanized and English
func (t *Transcriber) formatWithLlama(ctx context.Context, text string) (string, error) {
	// Create chat request
	chatReq := ChatRequest{
		Model: llamaModel,
		Messages: []ChatMessage{
			{Role: "user", Content: fmt.Sprintf(formatPrompt, text)},
		},
	}

	reqBody, err := json.Marshal(chatReq)
	if err != nil {
		return text, err // Return original on error
	}

	req, err := http.NewRequestWithContext(ctx, "POST", llamaAPIEndpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return text, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("Content-Type", "application/json")

	// Use retry logic for Llama API (shorter timeout)
	llamaClient := &http.Client{Timeout: llamaTimeout}
	resp, err := t.doWithRetry(ctx, req, llamaClient)
	if err != nil {
		return text, err // Return original on error
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return text, fmt.Errorf("llama API error: %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return text, err
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return text, err
	}

	if len(chatResp.Choices) == 0 {
		return text, nil
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// Transcribe sends an audio file to Groq's Whisper API and returns the transcribed text
// Uses the provided context for cancellation support
func (t *Transcriber) Transcribe(audioPath string) (string, error) {
	return t.TranscribeWithContext(context.Background(), audioPath)
}

// TranscribeWithContext sends an audio file to Groq's Whisper API with context support
func (t *Transcriber) TranscribeWithContext(ctx context.Context, audioPath string) (string, error) {
	// Check API key is not empty
	if t.apiKey == "" {
		return "", fmt.Errorf("API key is empty")
	}

	// Check context before starting
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Validate the audio file exists and check size
	fileInfo, err := os.Stat(audioPath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("audio file does not exist: %s", audioPath)
	}
	if err != nil {
		return "", fmt.Errorf("failed to stat audio file: %w", err)
	}

	// Check file size
	fileSize := fileInfo.Size()
	if fileSize < minAudioFileSize {
		return "", fmt.Errorf("audio file is too small: %d bytes (minimum: %d)", fileSize, minAudioFileSize)
	}
	if fileSize > maxAudioFileSize {
		return "", fmt.Errorf("audio file is too large: %d bytes (maximum: %d)", fileSize, maxAudioFileSize)
	}

	// Open the audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add the audio file to the form
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	// Use io.LimitReader to prevent reading more than maxAudioFileSize
	if _, err := io.Copy(part, io.LimitReader(file, maxAudioFileSize)); err != nil {
		return "", fmt.Errorf("failed to copy file data: %w", err)
	}

	// Add the model field
	if err := writer.WriteField("model", whisperModel); err != nil {
		return "", fmt.Errorf("failed to write model field: %w", err)
	}

	// Close the multipart writer
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create the HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", groqAPIEndpoint, body)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.apiKey))
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request with retry
	resp, err := t.doWithRetry(ctx, req, t.httpClient)
	if err != nil {
		return "", fmt.Errorf("network error: failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body with size limit
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err != nil {
			// Fallback to raw response if JSON parsing fails
			return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
		}
		// Use error message if available, otherwise fallback to raw response
		if errResp.Error.Message != "" {
			return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse the successful response
	var transcriptionResp TranscriptionResponse
	if err := json.Unmarshal(respBody, &transcriptionResp); err != nil {
		return "", fmt.Errorf("failed to parse response JSON: %w", err)
	}

	// Format with Llama to add separator between romanized and English
	formatted, err := t.formatWithLlama(ctx, transcriptionResp.Text)
	if err != nil {
		// Return original text if formatting fails (don't expose error to user)
		return transcriptionResp.Text, nil
	}

	return formatted, nil
}
