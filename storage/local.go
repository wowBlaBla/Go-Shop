package storage

import (
	"fmt"
	"github.com/google/logger"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func NewLocalStorage(root string, quality int) (*LocalStorage, error){
	local := &LocalStorage{
		root: root,
		quality: quality,
	}
	var err error
	return local, err
}

type LocalStorage struct {
	root string
	quality int
}

func (local *LocalStorage) copy(src, dst string) error {
	fi1, err := os.Stat(src)
	if err != nil {
		return err
	}
	if fi2, err := os.Stat(dst); err == nil {
		if fi1.ModTime().Equal(fi2.ModTime()) && fi1.Size() == fi2.Size() {
			//logger.Infof("ModTime and Size are the same, skip")
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if _, err := os.Stat(path.Dir(dst)); err != nil {
		if err = os.MkdirAll(path.Dir(dst), 0755); err != nil {
			return err
		}
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	err = out.Close()
	if err != nil {
		return err
	}
	return os.Chtimes(dst, fi1.ModTime(), fi1.ModTime())
}

// PutFile src - full local path to file, dst - relative path to file
func (local *LocalStorage) PutFile(src, location string) (string, error) {
	for _, suffix := range []string{"public", "static"} {
		if err := local.copy(src, path.Join(local.root, location, suffix)); err != nil {
			return location, err
		}
	}
	return location, nil
}

func (local *LocalStorage) DeleteFile(location string) error {
	for _, suffix := range []string{"public", "static"} {
		if err := os.RemoveAll(path.Join(local.root, suffix, location)); err != nil {
			return err
		}
	}
	return nil
}

func (local *LocalStorage) PutImage(src, location, sizes string) ([]string, error) {
	logger.Infof("PutImages: %+v, %+v, %+v", src, location, sizes)
	var locations []string
	for _, suffix := range []string{"public", "static"} {
		if err := local.copy(src, path.Join(local.root, suffix, location)); err != nil {
			return locations, err
		}
	}
	locations = append(locations, "/" + location)
	//
	if sizes != "" {
		fi1, err := os.Stat(src)
		if err != nil {
			return locations, err
		}
		var img image.Image
		ext := strings.ToLower(filepath.Ext(src))
		for _, suffix := range []string{"public", "static"} {
			if p := path.Join(local.root, suffix, path.Join(path.Dir(location), "resize")); len(p) > 0 {
				if _, err := os.Stat(p); err != nil {
					if err = os.MkdirAll(p, 0755); err != nil {
						logger.Warningf("%v", err)
					}
				}
			}
		}
		for _, size := range strings.Split(sizes, ",") {
			pair := strings.Split(size, "x")
			var width int
			if width, err = strconv.Atoi(pair[0]); err != nil {
				return locations, err
			}
			var height int
			if height, err = strconv.Atoi(pair[1]); err != nil {
				return locations, err
			}
			filename := path.Base(src)
			filename = filename[:len(filename) - len(filepath.Ext(filename))]
			filename = fmt.Sprintf("%s_%dx%d%s", filename, width, height, filepath.Ext(src))
			//
			dst := path.Join(local.root, "static", path.Join(path.Dir(location), "resize", filename))
			//
			//dst := path.Join(path.Dir(src), "resize", filename)
			if fi2, err := os.Stat(dst); err != nil || !fi1.ModTime().Equal(fi2.ModTime()) {
				if img == nil {
					file, err := os.Open(src)
					if err != nil {
						return locations, err
					}
					if ext == ".jpg" || ext == ".jpeg" {
						img, err = jpeg.Decode(file)
						if err != nil {
							return locations, err
						}
					}else if ext == ".png" {
						img, err = png.Decode(file)
						if err != nil {
							return locations, err
						}
					}
					file.Close()
				}
				m := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
				out, err := os.Create(dst)
				if err != nil {
					return locations, err
				}
				if ext == ".jpg" || ext == ".jpeg" {
					if err = jpeg.Encode(out, m, &jpeg.Options{Quality: local.quality}); err != nil {
						return locations, err
					}
					out.Close()
				}else if ext == ".png" {
					if err = png.Encode(out, m); err != nil {
						return locations, err
					}
					out.Close()
				}
				if err = os.Chtimes(dst, fi1.ModTime(), fi1.ModTime()); err != nil {
					return locations, err
				}
				if err := local.copy(dst, path.Join(local.root, "public", path.Join(path.Dir(location), "resize", filename))); err != nil {
					logger.Warningf("%+v", err)
				}
			}
			locations = append(locations, fmt.Sprintf("/%s %dw", path.Join(path.Dir(location), "resize", filename), width))
		}
	}
	return locations, nil
}

func (local *LocalStorage) DeleteImage(location string, sizes string) error {
	var err error
	for _, suffix := range []string{"public", "static"} {
		for _, size := range strings.Split(sizes, ",") {
			pair := strings.Split(size, "x")
			var width int
			if width, err = strconv.Atoi(pair[0]); err != nil {
				return err
			}
			var height int
			if height, err = strconv.Atoi(pair[1]); err != nil {
				return err
			}
			filename := path.Base(location)
			filename = filename[:len(filename)-len(filepath.Ext(filename))]
			filename = fmt.Sprintf("%s_%dx%d%s", filename, width, height, filepath.Ext(location))
			//
			if err := os.RemoveAll(path.Join(local.root, suffix, path.Join(path.Dir(location), "resize", filename))); err != nil {
				logger.Warningf("%+v", err)
			}
		}
		if err := os.RemoveAll(path.Join(local.root, suffix, location)); err != nil {
			return err
		}
	}
	return nil
}

func (local *LocalStorage) ImageResize(src, sizes string) ([]Image, error) {
	logger.Infof("ImageResize: %+v, %+v", src, sizes)
	var images []Image
	fi1, err := os.Stat(src)
	if err != nil {
		return images, err
	}
	var img image.Image
	if p := path.Join(path.Dir(src), "resize"); len(p) > 0 {
		if _, err := os.Stat(p); err != nil {
			if err = os.MkdirAll(p, 0755); err != nil {
				logger.Warningf("%v", err)
			}
		}
	}
	for _, size := range strings.Split(sizes, ",") {
		pair := strings.Split(size, "x")
		var width int
		if width, err = strconv.Atoi(pair[0]); err != nil {
			return images, err
		}
		var height int
		if height, err = strconv.Atoi(pair[1]); err != nil {
			return images, err
		}
		filename := path.Base(src)
		filename = filename[:len(filename) - len(filepath.Ext(filename))]
		filename = fmt.Sprintf("%s_%dx%d%s", filename, width, height, filepath.Ext(src))
		if fi2, err := os.Stat(path.Join(path.Dir(src), "resize", filename)); err != nil || !fi1.ModTime().Equal(fi2.ModTime()) {
			if img == nil {
				file, err := os.Open(src)
				if err != nil {
					return images, err
				}
				img, err = jpeg.Decode(file)
				if err != nil {
					return images, err
				}
				file.Close()
			}
			m := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
			out, err := os.Create(path.Join(path.Dir(src), "resize", filename))
			if err != nil {
				return images, err
			}
			if err = jpeg.Encode(out, m, &jpeg.Options{Quality: local.quality}); err != nil {
				return images, err
			}
			out.Close()
			if err = os.Chtimes(path.Join(path.Dir(src), "resize", filename), fi1.ModTime(), fi1.ModTime()); err != nil {
				logger.Warningf("%+v", err)
			}
		}
		images = append(images, Image{Filename: filename, Size: fmt.Sprintf("%dw", width)})
	}
	return images, nil
}