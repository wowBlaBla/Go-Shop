package storage

type Storage interface {
	PutFile(src, location string) (string, error)
	PutImage(src, location, sizes string) ([]string, error)
	//
	//Copy(src, dst string) error
	//ImageResize(src, sizes string) ([]Image, error)
}

type Image struct {
	Filename string
	Size string
}
