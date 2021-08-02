package storage

type Storage interface {
	Open() error
	PutFile(src, location string) (string, error)
	DeleteFile(location string) error
	PutImage(src, location, sizes string) ([]string, error)
	DeleteImage(location, sizes string) error
	Close() error
	//
	//Copy(src, dst string) error
	//ImageResize(src, sizes string) ([]Image, error)
}

type Image struct {
	Filename string
	Size string
}
