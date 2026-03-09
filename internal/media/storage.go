package media

// Storage is the media file storage interface.
type Storage interface {
	// SaveFile persists a file from srcPath. Returns media ID.
	SaveFile(sessionKey, srcPath, mimeType string) (id string, err error)

	// LoadPath returns a local file path for the media ID.
	// For cloud storage, downloads to a temp file and returns the temp path.
	LoadPath(id string) (string, error)

	// PublicURL returns a publicly accessible URL for the media ID.
	// Returns "" for local storage (served via gateway HTTP).
	PublicURL(id string) string

	// DeleteSession removes all media for a session.
	DeleteSession(sessionKey string) error
}
