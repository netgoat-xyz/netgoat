package tlsmanager

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/acme/autocert"
)

// CacheKeyEnvironment is the environment variable that supplies the AES-256
// key used to encrypt ACME account and certificate cache entries.
const CacheKeyEnvironment = "NETGOAT_ACME_CACHE_KEY"

var encryptedCacheMagic = []byte("NETGOAT-ACME-CACHE-1")

// EncryptedCache is an autocert.Cache backed by AES-256-GCM encrypted files.
// File names are hashes of autocert's cache keys, so neither certificate data
// nor cache key names are written to disk in plaintext.
type EncryptedCache struct {
	directory string
	aead      cipher.AEAD
}

var _ autocert.Cache = (*EncryptedCache)(nil)

// NewEncryptedCache creates an encrypted ACME cache using a 32-byte AES-256
// key. The directory is created with owner-only permissions when necessary.
func NewEncryptedCache(directory string, key []byte) (*EncryptedCache, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("ACME cache key must be 32 bytes, got %d", len(key))
	}
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil, errors.New("ACME cache directory is required")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create ACME cache directory: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("initialize ACME cache cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("initialize ACME cache AEAD: %w", err)
	}
	return &EncryptedCache{
		directory: filepath.Clean(directory),
		aead:      aead,
	}, nil
}

// NewEncryptedCacheFromEnv creates an encrypted cache using the base64-encoded
// AES-256 key in NETGOAT_ACME_CACHE_KEY.
func NewEncryptedCacheFromEnv(directory string) (*EncryptedCache, error) {
	return NewEncryptedCacheFromBase64(directory, os.Getenv(CacheKeyEnvironment))
}

// NewEncryptedCacheFromBase64 creates an encrypted cache from a base64-encoded
// AES-256 key. Standard and URL-safe base64, with or without padding, are
// accepted to make environment injection reliable across deployment systems.
func NewEncryptedCacheFromBase64(directory, encodedKey string) (*EncryptedCache, error) {
	encodedKey = strings.TrimSpace(encodedKey)
	if encodedKey == "" {
		return nil, fmt.Errorf("%s is required", CacheKeyEnvironment)
	}

	var (
		key []byte
		err error
	)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		key, err = encoding.DecodeString(encodedKey)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", CacheKeyEnvironment, err)
	}
	defer clearBytes(key)
	return NewEncryptedCache(directory, key)
}

// Get retrieves and authenticates a cache entry. A missing entry returns
// autocert.ErrCacheMiss; malformed or modified entries return an error rather
// than being treated as a cache miss.
func (c *EncryptedCache) Get(ctx context.Context, key string) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if c == nil || c.aead == nil {
		return nil, errors.New("encrypted ACME cache is not initialized")
	}

	encoded, err := os.ReadFile(c.pathFor(key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, autocert.ErrCacheMiss
		}
		return nil, fmt.Errorf("read encrypted ACME cache entry: %w", err)
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	nonceSize := c.aead.NonceSize()
	minimumSize := len(encryptedCacheMagic) + nonceSize + c.aead.Overhead()
	if len(encoded) < minimumSize || !bytes.Equal(encoded[:len(encryptedCacheMagic)], encryptedCacheMagic) {
		return nil, errors.New("encrypted ACME cache entry has an invalid format")
	}
	nonceStart := len(encryptedCacheMagic)
	nonceEnd := nonceStart + nonceSize
	plaintext, err := c.aead.Open(nil, encoded[nonceStart:nonceEnd], encoded[nonceEnd:], cacheAssociatedData(key))
	if err != nil {
		return nil, fmt.Errorf("authenticate encrypted ACME cache entry: %w", err)
	}
	return plaintext, nil
}

// Put encrypts and atomically persists a cache entry with owner-only file
// permissions. The cache key is authenticated as associated data, preventing
// a valid encrypted entry from being replayed under another cache key.
func (c *EncryptedCache) Put(ctx context.Context, key string, data []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if c == nil || c.aead == nil {
		return errors.New("encrypted ACME cache is not initialized")
	}
	if err := os.MkdirAll(c.directory, 0o700); err != nil {
		return fmt.Errorf("create ACME cache directory: %w", err)
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generate ACME cache nonce: %w", err)
	}
	ciphertext := c.aead.Seal(nil, nonce, data, cacheAssociatedData(key))
	encoded := make([]byte, 0, len(encryptedCacheMagic)+len(nonce)+len(ciphertext))
	encoded = append(encoded, encryptedCacheMagic...)
	encoded = append(encoded, nonce...)
	encoded = append(encoded, ciphertext...)

	temporaryFile, err := os.CreateTemp(c.directory, ".netgoat-acme-")
	if err != nil {
		return fmt.Errorf("create ACME cache temporary file: %w", err)
	}
	temporaryName := temporaryFile.Name()
	committed := false
	defer func() {
		_ = temporaryFile.Close()
		if !committed {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporaryFile.Chmod(0o600); err != nil {
		return fmt.Errorf("set ACME cache file permissions: %w", err)
	}
	if _, err := temporaryFile.Write(encoded); err != nil {
		return fmt.Errorf("write encrypted ACME cache entry: %w", err)
	}
	if err := temporaryFile.Sync(); err != nil {
		return fmt.Errorf("sync encrypted ACME cache entry: %w", err)
	}
	if err := temporaryFile.Close(); err != nil {
		return fmt.Errorf("close encrypted ACME cache entry: %w", err)
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, c.pathFor(key)); err != nil {
		return fmt.Errorf("publish encrypted ACME cache entry: %w", err)
	}
	committed = true
	return nil
}

// Delete removes a cache entry. Deleting an entry that does not exist succeeds.
func (c *EncryptedCache) Delete(ctx context.Context, key string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if c == nil || c.aead == nil {
		return errors.New("encrypted ACME cache is not initialized")
	}
	if err := os.Remove(c.pathFor(key)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete encrypted ACME cache entry: %w", err)
	}
	return nil
}

func (c *EncryptedCache) pathFor(key string) string {
	digest := sha256.Sum256([]byte(key))
	return filepath.Join(c.directory, hex.EncodeToString(digest[:])+".cache")
}

func cacheAssociatedData(key string) []byte {
	associatedData := make([]byte, 0, len(encryptedCacheMagic)+len(key))
	associatedData = append(associatedData, encryptedCacheMagic...)
	associatedData = append(associatedData, key...)
	return associatedData
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
