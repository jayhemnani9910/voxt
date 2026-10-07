package notify

import (
	"log"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"
)

// Notifier handles desktop notifications via DBus
// Close() must be called to release resources
type Notifier struct {
	conn     *dbus.Conn
	notifier notify.Notifier
}

// NewNotifier creates a new DBus notifier
// Caller must call Close() when done
func NewNotifier() (*Notifier, error) {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, err
	}

	if err = conn.Auth(nil); err != nil {
		conn.Close()
		return nil, err
	}

	if err = conn.Hello(); err != nil {
		conn.Close()
		return nil, err
	}

	notifier, err := notify.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &Notifier{
		conn:     conn,
		notifier: notifier,
	}, nil
}

// truncateUTF8 safely truncates a string to maxRunes runes without breaking UTF-8
func truncateUTF8(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}

// ShowTranscription shows a notification with the transcribed text
// The notification auto-dismisses after 3 seconds
func (n *Notifier) ShowTranscription(text string) error {
	// Truncate text for notification body if too long (rune-aware)
	displayText := truncateUTF8(text, 197)

	notification := notify.Notification{
		AppName:       "Voxt",
		Summary:       "Transcription Complete",
		Body:          displayText,
		ExpireTimeout: 3000, // 3 seconds
	}

	_, err := n.notifier.SendNotification(notification)
	if err != nil {
		return err
	}

	// Length only: the text is whatever the user dictated.
	log.Printf("Notification shown (%d chars)", len(displayText))
	return nil
}

// ShowError shows an error notification
func (n *Notifier) ShowError(message string) error {
	notification := notify.Notification{
		AppName:       "Voxt",
		Summary:       "Error",
		Body:          message,
		ExpireTimeout: 5000, // 5 seconds for errors
	}

	_, err := n.notifier.SendNotification(notification)
	return err
}

// Close closes the DBus connection
func (n *Notifier) Close() error {
	return n.conn.Close()
}
