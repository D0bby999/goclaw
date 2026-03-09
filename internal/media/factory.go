package media

import (
	"fmt"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// NewStorage creates a Storage backend based on config.
// Priority: S3 > R2 > Local. Returns (storage, providerName, error).
func NewStorage(mediaCfg *config.MediaConfig, localDir string) (Storage, string, error) {
	// S3 first
	if mediaCfg != nil && mediaCfg.S3 != nil && mediaCfg.S3.Bucket != "" {
		s3cfg := toS3Config(mediaCfg.S3)
		store, err := NewS3Storage(s3cfg)
		if err != nil {
			return nil, "", fmt.Errorf("media: create s3 storage: %w", err)
		}
		return store, "s3", nil
	}

	// R2 second
	if mediaCfg != nil && mediaCfg.R2 != nil && mediaCfg.R2.Bucket != "" {
		s3cfg := r2ToS3Config(mediaCfg.R2)
		store, err := NewS3Storage(s3cfg)
		if err != nil {
			return nil, "", fmt.Errorf("media: create r2 storage: %w", err)
		}
		return store, "r2", nil
	}

	// Local fallback
	store, err := NewLocalStorage(localDir)
	if err != nil {
		return nil, "", fmt.Errorf("media: create local storage: %w", err)
	}
	return store, "local", nil
}

// parseTTL parses a duration string with a fallback default of 1h.
func parseTTL(s string) time.Duration {
	if s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return time.Hour
}

func toS3Config(cfg *config.MediaS3Config) S3Config {
	return S3Config{
		Bucket:         cfg.Bucket,
		Region:         cfg.Region,
		Endpoint:       cfg.Endpoint,
		AccessKeyID:    cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		Prefix:         cfg.Prefix,
		PublicURL:      cfg.PublicURL,
		URLExpiry:      parseTTL(cfg.URLExpiry),
		ForcePathStyle: cfg.ForcePathStyle,
	}
}

func r2ToS3Config(cfg *config.MediaR2Config) S3Config {
	endpoint := ""
	if cfg.AccountID != "" {
		endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	}
	return S3Config{
		Bucket:         cfg.Bucket,
		Region:         "auto", // R2 uses "auto" region
		Endpoint:       endpoint,
		AccessKeyID:    cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		Prefix:         cfg.Prefix,
		PublicURL:      cfg.PublicURL,
		URLExpiry:      parseTTL(cfg.URLExpiry),
	}
}
