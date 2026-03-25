package media

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

const s3Timeout = 30 * time.Second

// S3Config configures S3-compatible storage (AWS S3, Cloudflare R2, MinIO).
type S3Config struct {
	Bucket         string
	Region         string
	Endpoint       string        // custom endpoint (R2, MinIO, etc.)
	AccessKeyID    string
	SecretAccessKey string
	Prefix         string        // object key prefix (e.g. "media/")
	PublicURL      string        // public base URL for direct access
	URLExpiry      time.Duration // presigned URL TTL (default 1h)
	ForcePathStyle bool          // for MinIO/non-AWS endpoints
}

// S3Storage implements Storage for S3-compatible backends.
type S3Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	prefix    string
	publicURL string
	urlTTL    time.Duration
	keyCache  sync.Map // mediaID → S3 key (populated on SaveFile, used by LoadPath/PublicURL)
}

// NewS3Storage creates an S3Storage from the given config.
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("media/s3: bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.URLExpiry <= 0 {
		cfg.URLExpiry = time.Hour
	}

	opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.Region = cfg.Region
			if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
				o.Credentials = credentials.NewStaticCredentialsProvider(
					cfg.AccessKeyID, cfg.SecretAccessKey, "",
				)
			}
			if cfg.Endpoint != "" {
				o.BaseEndpoint = aws.String(cfg.Endpoint)
			}
			if cfg.ForcePathStyle {
				o.UsePathStyle = true
			}
		},
	}

	client := s3.New(s3.Options{}, opts...)
	presigner := s3.NewPresignClient(client)

	return &S3Storage{
		client:    client,
		presigner: presigner,
		bucket:    cfg.Bucket,
		prefix:    cfg.Prefix,
		publicURL: strings.TrimRight(cfg.PublicURL, "/"),
		urlTTL:    cfg.URLExpiry,
	}, nil
}

// SaveFile uploads a file to S3. Returns media ID.
func (s *S3Storage) SaveFile(sessionKey, srcPath, mimeType string) (string, error) {
	mediaID := uuid.New().String()
	ext := ExtFromMime(mimeType)
	if ext == "" {
		ext = filepath.Ext(srcPath)
	}
	key := s.objectKey(sessionKey, mediaID+ext)

	f, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("media/s3: open source: %w", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &s.bucket,
		Key:         &key,
		Body:        f,
		ContentType: &mimeType,
	})
	if err != nil {
		return "", fmt.Errorf("media/s3: put object: %w", err)
	}

	s.keyCache.Store(mediaID, key)
	_ = os.Remove(srcPath)
	return mediaID, nil
}

// LoadPath downloads the file to a temp path and returns it.
func (s *S3Storage) LoadPath(id string) (string, error) {
	key, err := s.resolveKey(id)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return "", fmt.Errorf("media/s3: get object: %w", err)
	}
	defer resp.Body.Close()

	ext := filepath.Ext(key)
	tmp, err := os.CreateTemp("", "goclaw-media-*"+ext)
	if err != nil {
		return "", fmt.Errorf("media/s3: create temp file: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("media/s3: download: %w", err)
	}
	return tmp.Name(), nil
}

// PublicURL returns a presigned or custom-domain URL.
func (s *S3Storage) PublicURL(id string) string {
	key, err := s.resolveKey(id)
	if err != nil {
		return ""
	}

	if s.publicURL != "" {
		return s.publicURL + "/" + key
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	}, s3.WithPresignExpires(s.urlTTL))
	if err != nil {
		slog.Warn("media/s3: presign failed", "id", id, "error", err)
		return ""
	}
	return req.URL
}

// DeleteSession removes all objects with the session prefix (paginated).
func (s *S3Storage) DeleteSession(sessionKey string) error {
	prefix := s.sessionPrefix(sessionKey)
	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &prefix,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("media/s3: list objects: %w", err)
		}
		if len(page.Contents) == 0 {
			continue
		}

		objects := make([]types.ObjectIdentifier, len(page.Contents))
		for i, obj := range page.Contents {
			objects[i] = types.ObjectIdentifier{Key: obj.Key}
			// Evict from cache.
			base := filepath.Base(aws.ToString(obj.Key))
			if idx := strings.LastIndex(base, "."); idx > 0 {
				s.keyCache.Delete(base[:idx])
			}
		}

		_, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &s.bucket,
			Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return fmt.Errorf("media/s3: delete objects: %w", err)
		}
	}
	return nil
}

// objectKey constructs the full S3 key for a file.
func (s *S3Storage) objectKey(sessionKey, filename string) string {
	return s.sessionPrefix(sessionKey) + filename
}

// sessionPrefix returns the S3 key prefix for a session.
func (s *S3Storage) sessionPrefix(sessionKey string) string {
	h := sha256.Sum256([]byte(sessionKey))
	hash := fmt.Sprintf("%x", h[:6])
	return s.prefix + hash + "/"
}

// resolveKey returns the S3 key for a media ID.
// Uses in-memory cache first, falls back to paginated bucket listing.
func (s *S3Storage) resolveKey(id string) (string, error) {
	if key, ok := s.keyCache.Load(id); ok {
		return key.(string), nil
	}
	return s.findObjectKey(id)
}

// findObjectKey locates the full key for a media ID by paginated bucket listing.
func (s *S3Storage) findObjectKey(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	prefix := s.prefix
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &prefix,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("media/s3: list for lookup: %w", err)
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			base := filepath.Base(key)
			if strings.HasPrefix(base, id+".") || base == id {
				s.keyCache.Store(id, key)
				return key, nil
			}
		}
	}
	return "", fmt.Errorf("media/s3: file not found: %s", id)
}
