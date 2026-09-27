package objectstore

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"legal-bench-api/internal/config"
)

type Object struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
}

type Store interface {
	Put(ctx context.Context, key string, contentType string, data []byte) error
	Get(ctx context.Context, key string) (Object, error)
}

func New(cfg config.Config) (Store, error) {
	switch strings.ToLower(cfg.ObjectStorageProvider) {
	case "", "local":
		return LocalStore{Root: cfg.ObjectStorageLocalDir}, nil
	case "s3":
		store := S3Store{
			Endpoint:        strings.TrimRight(cfg.ObjectStorageEndpoint, "/"),
			Region:          cfg.ObjectStorageRegion,
			Bucket:          cfg.ObjectStorageBucket,
			AccessKeyID:     cfg.ObjectStorageAccessKeyID,
			SecretAccessKey: cfg.ObjectStorageSecretAccessKey,
			ForcePathStyle:  cfg.ObjectStorageForcePathStyle,
			Client:          http.DefaultClient,
		}
		if store.Endpoint == "" || store.Region == "" || store.Bucket == "" || store.AccessKeyID == "" || store.SecretAccessKey == "" {
			return nil, fmt.Errorf("s3 object storage requires endpoint, region, bucket, access key, and secret key")
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported object storage provider %q", cfg.ObjectStorageProvider)
	}
}

type LocalStore struct {
	Root string
}

func (s LocalStore) Put(ctx context.Context, key string, contentType string, data []byte) error {
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return err
	}
	path := filepath.Join(s.Root, filepath.FromSlash(cleanKey))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (s LocalStore) Get(ctx context.Context, key string) (Object, error) {
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return Object{}, err
	}
	filePath := filepath.Join(s.Root, filepath.FromSlash(cleanKey))
	file, err := os.Open(filePath)
	if err != nil {
		return Object{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return Object{}, err
	}
	return Object{Body: file, ContentLength: info.Size()}, nil
}

type S3Store struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
	Client          *http.Client
}

func (s S3Store) Put(ctx context.Context, key string, contentType string, data []byte) error {
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return err
	}
	request, err := s.newRequest(ctx, http.MethodPut, cleanKey, bytes.NewReader(data), int64(len(data)), contentType)
	if err != nil {
		return err
	}
	response, err := s.client().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("put object failed: status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (s S3Store) Get(ctx context.Context, key string) (Object, error) {
	cleanKey, err := cleanObjectKey(key)
	if err != nil {
		return Object{}, err
	}
	request, err := s.newRequest(ctx, http.MethodGet, cleanKey, nil, 0, "")
	if err != nil {
		return Object{}, err
	}
	response, err := s.client().Do(request)
	if err != nil {
		return Object{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return Object{}, fmt.Errorf("get object failed: status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return Object{Body: response.Body, ContentType: response.Header.Get("Content-Type"), ContentLength: response.ContentLength}, nil
}

func (s S3Store) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return http.DefaultClient
}

func (s S3Store) newRequest(ctx context.Context, method string, key string, body io.Reader, contentLength int64, contentType string) (*http.Request, error) {
	endpoint, err := url.Parse(s.Endpoint)
	if err != nil {
		return nil, err
	}

	objectPath := escapeObjectPath(key)
	if s.ForcePathStyle {
		endpoint.Path = path.Join(endpoint.Path, s.Bucket, objectPath)
	} else {
		endpoint.Host = s.Bucket + "." + endpoint.Host
		endpoint.Path = path.Join(endpoint.Path, objectPath)
	}

	var payload []byte
	if body != nil {
		payload, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(payload)
		contentLength = int64(len(payload))
	}
	payloadHash := sha256Hex(payload)
	if body == nil {
		payloadHash = sha256Hex(nil)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return nil, err
	}
	if contentLength > 0 {
		request.ContentLength = contentLength
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	request.Header.Set("x-amz-content-sha256", payloadHash)
	s.sign(request, payloadHash)
	return request, nil
}

func (s S3Store) sign(request *http.Request, payloadHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	credentialScope := dateStamp + "/" + s.Region + "/s3/aws4_request"

	request.Host = request.URL.Host
	request.Header.Set("Host", request.URL.Host)
	request.Header.Set("x-amz-date", amzDate)

	signedHeaders, canonicalHeaders := canonicalHeaders(request.Header)
	canonicalRequest := strings.Join([]string{
		request.Method,
		request.URL.EscapedPath(),
		request.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := awsSigningKey(s.SecretAccessKey, dateStamp, s.Region, "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.AccessKeyID+"/"+credentialScope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func canonicalHeaders(headers http.Header) (string, string) {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, strings.ToLower(key))
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		values := headers.Values(key)
		for i := range values {
			values[i] = strings.Join(strings.Fields(values[i]), " ")
		}
		builder.WriteString(key)
		builder.WriteString(":")
		builder.WriteString(strings.Join(values, ","))
		builder.WriteString("\n")
	}
	return strings.Join(keys, ";"), builder.String()
}

func awsSigningKey(secret string, date string, region string, service string) []byte {
	dateKey := hmacSHA256([]byte("AWS4"+secret), date)
	dateRegionKey := hmacSHA256(dateKey, region)
	dateRegionServiceKey := hmacSHA256(dateRegionKey, service)
	return hmacSHA256(dateRegionServiceKey, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func cleanObjectKey(key string) (string, error) {
	cleaned := path.Clean("/" + key)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("invalid object key")
	}
	return cleaned, nil
}

func escapeObjectPath(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}
