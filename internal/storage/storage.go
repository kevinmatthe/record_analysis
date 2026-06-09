package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type StoredObject struct {
	RelationshipID string `json:"relationship_id"`
	ObjectKey      string `json:"object_key"`
	URI            string `json:"uri"`
	ContentHash    string `json:"content_hash"`
}

type ObjectStore interface {
	PutChatFile(path string, relationshipID string) (StoredObject, error)
}

type EndpointConfig struct {
	Endpoint string `json:"endpoint" yaml:"endpoint" toml:"endpoint"`
	UseSSL   bool   `json:"use_ssl" yaml:"use_ssl" toml:"use_ssl"`
}

type MinioConfig struct {
	Internal   EndpointConfig `json:"internal" yaml:"internal" toml:"internal"`
	External   EndpointConfig `json:"external" yaml:"external" toml:"external"`
	AccessKey  string         `json:"ak" yaml:"ak" toml:"ak"`
	SecretKey  string         `json:"sk" yaml:"sk" toml:"sk"`
	ExpireTime string         `json:"expire_time" yaml:"expire_time" toml:"expire_time"`
	Bucket     string         `json:"bucket" yaml:"bucket" toml:"bucket"`
}

type LocalObjectStore struct {
	root string
}

func NewLocalObjectStore(root string) *LocalObjectStore {
	return &LocalObjectStore{root: root}
}

func (s *LocalObjectStore) PutChatFile(path string, relationshipID string) (StoredObject, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StoredObject{}, err
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	objectKey := fmt.Sprintf("relationships/%s/uploads/%s%s", relationshipID, digest[:16], stringsToLowerExt(path))
	target := filepath.Join(s.root, filepath.FromSlash(objectKey))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return StoredObject{}, err
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return StoredObject{}, err
	}
	return StoredObject{
		RelationshipID: relationshipID,
		ObjectKey:      objectKey,
		URI:            target,
		ContentHash:    digest,
	}, nil
}

func stringsToLowerExt(path string) string {
	return strings.ToLower(filepath.Ext(filepath.Base(path)))
}
