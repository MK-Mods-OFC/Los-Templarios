package storage

import (
	"io"

	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/config"
)

// Storage interface provides functionalities to
// access an object storage driver.
type Storage interface {
	Connect(cfg config.Provider) error

	BucketExists(name string) (bool, error)
	CreateBucket(name string, location ...string) error
	CreateBucketIfNotExists(name string, location ...string) error

	PutObject(bucketName, objectName string, reader io.Reader, objectSize int64, mimeType string) error
	GetObject(bucketName, objectName string) (io.ReadCloser, int64, error)
	DeleteObject(bucketName, objectName string) error
}
