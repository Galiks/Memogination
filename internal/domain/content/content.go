// Package content defines the meme and situation content domain models.
package content

import "time"

// Meme is an uploaded image used as game content.
type Meme struct {
	ID               string    `json:"id"`
	OriginalPath     string    `json:"originalPath"`
	ScreenPath       string    `json:"screenPath"`
	ThumbnailPath    string    `json:"thumbnailPath"`
	OriginalFilename string    `json:"originalFilename"`
	MimeType         string    `json:"mimeType"`
	SHA256           string    `json:"sha256"`
	Enabled          bool      `json:"enabled"`
	Source           string    `json:"source"`
	CreatedAt        time.Time `json:"createdAt"`
}

// Situation is a text prompt used to seed a round.
type Situation struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Enabled   bool      `json:"enabled"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

// ContentPack is a named collection of content.
type ContentPack struct {
	ID   string
	Name string
}
