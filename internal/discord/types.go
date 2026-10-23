// Package discord implements the Discord Bot Gateway bridge.
package discord

// TextChunkLimit is the maximum characters per Discord message.
const TextChunkLimit = 2000

// Attachment is a file on an inbound message. URL is the CDN address.
type Attachment struct {
	URL         string
	Name        string
	ContentType string
	Size        int
}

// Message is a normalised inbound Discord DM.
type Message struct {
	ChannelID   string
	AuthorID    string
	Content     string
	Attachments []Attachment
}
