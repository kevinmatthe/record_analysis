package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultMinioBucket = "record-analysis"

type MinioObjectStore struct {
	internal *minio.Client
	external *minio.Client
	config   MinioConfig
}

func (c MinioConfig) Enabled() bool {
	return c.Internal.Endpoint != "" && c.External.Endpoint != "" && c.AccessKey != "" && c.SecretKey != ""
}

func NewMinioObjectStore(config MinioConfig) (*MinioObjectStore, error) {
	if !config.Enabled() {
		return nil, errors.New("minio config missing or incomplete")
	}
	if config.Bucket == "" {
		config.Bucket = defaultMinioBucket
	}
	internal, err := minio.New(config.Internal.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.Internal.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	external, err := minio.New(config.External.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.External.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	return &MinioObjectStore{internal: internal, external: external, config: config}, nil
}

func (s *MinioObjectStore) PutChatFile(path string, relationshipID string) (StoredObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredObject{}, err
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	objectKey := fmt.Sprintf("relationships/%s/uploads/%s%s", relationshipID, digest[:16], stringsToLowerExt(path))
	if err := s.ensureBucket(context.Background()); err != nil {
		return StoredObject{}, err
	}
	_, err = s.internal.FPutObject(
		context.Background(),
		s.config.Bucket,
		objectKey,
		path,
		minio.PutObjectOptions{ContentType: contentTypeForExt(filepath.Ext(path))},
	)
	if err != nil {
		return StoredObject{}, err
	}
	uri := fmt.Sprintf("minio://%s/%s", s.config.Bucket, objectKey)
	if signed, err := s.presign(objectKey); err == nil && signed != "" {
		uri = signed
	}
	return StoredObject{
		RelationshipID: relationshipID,
		ObjectKey:      objectKey,
		URI:            uri,
		ContentHash:    digest,
	}, nil
}

func (s *MinioObjectStore) ensureBucket(ctx context.Context) error {
	found, err := s.internal.BucketExists(ctx, s.config.Bucket)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	return s.internal.MakeBucket(ctx, s.config.Bucket, minio.MakeBucketOptions{})
}

func (s *MinioObjectStore) presign(objectKey string) (string, error) {
	expire := 7 * 24 * time.Hour
	if s.config.ExpireTime != "" {
		if parsed, err := time.ParseDuration(s.config.ExpireTime); err == nil {
			expire = parsed
		}
	}
	u, err := s.external.PresignedGetObject(context.Background(), s.config.Bucket, objectKey, expire, url.Values{})
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func contentTypeForExt(ext string) string {
	switch ext {
	case ".json":
		return "application/json; charset=utf-8"
	case ".csv":
		return "text/csv; charset=utf-8"
	default:
		return "text/plain; charset=utf-8"
	}
}
