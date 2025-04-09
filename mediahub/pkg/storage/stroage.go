package storage

import "io"

type Storage interface {
	Upload(r io.Reader, md5Digest []byte, dstPath string) (url string, err error)
}

type StorageFactory interface {
	CreateStorage() Storage
}
