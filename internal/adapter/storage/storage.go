// Package storage stores private objects in MinIO and hands out short-lived
// presigned URLs (docs/12 §3).
//
// Nothing here is ever served from the application origin, and no client
// filename is ever used on disk: payment proofs are financial evidence and
// uploads are the classic path to stored XSS.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/stevenwilliam/ruuma/internal/platform/apierror"

	_ "golang.org/x/image/webp" // decode WebP uploads
)

// Limits from docs/12 §3.
const (
	MaxProofBytes = 5 << 20 // 5 MB
	MaxPhotoBytes = 8 << 20 // 8 MB
	MaxDimension  = 4000    // decompression-bomb guard
)

var (
	ErrTooLarge     = errors.New("storage: file is too large")
	ErrUnsupported  = errors.New("storage: unsupported file type")
	ErrTooLargeDims = errors.New("storage: image dimensions are too large")
)

// Config mirrors the MinIO settings in .env.example.
type Config struct {
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	Bucket         string
	UseSSL         bool
}

// Client wraps MinIO with ruuma's upload rules.
type Client struct {
	mc     *minio.Client
	bucket string
}

func New(ctx context.Context, cfg Config) (*Client, error) {
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: client: %w", err)
	}

	exists, err := mc.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("storage: bucket check: %w", err)
	}
	if !exists {
		if err := mc.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("storage: create bucket: %w", err)
		}
	}
	// The bucket keeps MinIO's default private policy: objects are reachable
	// only through a presigned URL (docs/12, A05).
	return &Client{mc: mc, bucket: cfg.Bucket}, nil
}

// Kind selects the ruleset for an upload. Menu photos follow the same rules as
// payment proofs — generated keys, magic-byte checks, re-encoding, presigned
// access only (BR-2.2.8, BR-2.6.11).
type Kind string

const (
	KindProof Kind = "proof" // payment proof: images and PDF
	KindPhoto Kind = "photo" // menu photo: images only, re-encoded
)

// Put validates and stores an object, returning its generated key.
//
// The content type is decided by sniffing magic bytes, never by the client's
// filename or declared type; images are re-encoded, which strips EXIF and kills
// polyglot files (docs/12 §3).
func (c *Client) Put(ctx context.Context, kind Kind, prefix string, data []byte) (string, error) {
	limit := MaxProofBytes
	if kind == KindPhoto {
		limit = MaxPhotoBytes
	}
	if len(data) == 0 {
		return "", apierror.Validation("The file is empty.", nil)
	}
	if len(data) > limit {
		return "", apierror.Validation("That file is too large.",
			map[string]any{"max_bytes": limit})
	}

	mime, ext, err := sniff(data)
	if err != nil {
		return "", apierror.Validation("That file type is not accepted.",
			map[string]any{"accepted": "JPEG, PNG, WebP" + pdfNote(kind)})
	}
	if mime == "application/pdf" && kind != KindProof {
		return "", apierror.Validation("Menu photos must be images.", nil)
	}

	body := data
	if mime != "application/pdf" {
		reencoded, err := reencode(data)
		if err != nil {
			return "", apierror.Validation("That image could not be processed.", nil)
		}
		body = reencoded
		mime, ext = "image/jpeg", "jpg"
	}

	key := fmt.Sprintf("%s/%s/%s.%s", prefix, time.Now().UTC().Format("2006/01"), uuid.New(), ext)
	_, err = c.mc.PutObject(ctx, c.bucket, key, bytes.NewReader(body), int64(len(body)),
		minio.PutObjectOptions{
			ContentType: mime,
			// Proofs are downloads, never rendered inline (docs/12 §3).
			ContentDisposition: "attachment",
			UserMetadata:       map[string]string{"x-amz-meta-kind": string(kind)},
		})
	if err != nil {
		return "", fmt.Errorf("storage: put: %w", err)
	}
	return key, nil
}

// PresignGet returns a short-lived download URL. TTL is deliberately short:
// these URLs end up in browser history and support tickets.
func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 || ttl > time.Hour {
		ttl = 10 * time.Minute
	}
	params := url.Values{}
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, ttl, params)
	if err != nil {
		return "", fmt.Errorf("storage: presign: %w", err)
	}
	return u.String(), nil
}

// Get reads an object back, used by tests and by the ticket printer.
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = obj.Close() }()
	return io.ReadAll(obj)
}

// Delete removes an object.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// sniff identifies a file by its magic bytes (docs/12 §3). An extension is a
// claim; the first bytes are evidence.
func sniff(data []byte) (mime, ext string, err error) {
	switch {
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg", "jpg", nil
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", "png", nil
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp", "webp", nil
	case len(data) >= 5 && bytes.Equal(data[:5], []byte("%PDF-")):
		return "application/pdf", "pdf", nil
	default:
		return "", "", ErrUnsupported
	}
}

// reencode decodes and re-encodes an image, which strips metadata and refuses
// anything that only pretends to be an image.
func reencode(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		return nil, ErrTooLargeDims
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func pdfNote(kind Kind) string {
	if kind == KindProof {
		return " or PDF"
	}
	return ""
}

// register the PNG decoder for image.Decode
var _ = png.Decode
