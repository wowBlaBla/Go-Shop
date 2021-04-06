package storage

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/logger"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	"image/png"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reUrl = regexp.MustCompile(`https?://([^\/]+)(/.+)`)
)

func NewAWSS3Storage(accessKeyID, secretAccessKey, region, bucket, prefix string, temp string, quality int, rewrite string) (*AWSS3Storage, error){
	storage := &AWSS3Storage{
		prefix: prefix,
		quality: quality,
		rewrite: rewrite,
	}
	var err error
	storage.session, err = session.NewSession(
		&aws.Config{
			Region: aws.String(region),
			Credentials: credentials.NewStaticCredentials(
				accessKeyID,
				secretAccessKey,
				"", // a token will be created when the session it's used.
			),
		})
	if err != nil {
		return storage, err
	}
	storage.Bucket = bucket
	if _, err := os.Stat(temp); err != nil {
		if err = os.MkdirAll(temp, 0755); err != nil {
			return storage, err
		}
	}
	storage.temp = temp
	//
	return storage, err
}

type AWSS3Storage struct {
	session *session.Session
	AccessKeyID string
	SecretAccessKey string
	Region string
	Bucket string
	prefix string
	temp string // full/path
	quality int
	rewrite string
}

type AWSS3StorageItem struct {
	Created time.Time
	Url string
	Size int64
	Modified time.Time
}

func (storage *AWSS3Storage) upload(src, location string) (string, error) {
	//logger.Infof("upload: %+v, %+v", src, location)
	var url string
	uploader := s3manager.NewUploader(storage.session)

	file, err := os.Open(src)
	if err != nil {
		return url, err
	}

	if up, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(storage.Bucket),
		ACL:    aws.String("public-read"),
		Key:    aws.String(path.Join(storage.prefix, location)),
		Body:   file,
	}); err == nil {
		url = up.Location
	} else {
		return url, err
	}

	return url, err
}

func (storage *AWSS3Storage) delete(location string) error {
	svc := s3.New(storage.session)
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(storage.Bucket),
		Key:    aws.String(location),
	}

	_, err := svc.DeleteObject(input)
	if err != nil {
		if err, ok := err.(awserr.Error); ok {
			switch err.Code() {
			default:
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func (storage *AWSS3Storage) rw(link string) string {
	if storage.rewrite != "" {
		if res := reUrl.FindAllStringSubmatch(link, 1); len(res) > 0 && len(res[0]) > 3 {
			return storage.rewrite + res[0][2]
		}
	}
	return link
}

func (storage *AWSS3Storage) PutFile(src, location string) (string, error) {
	return storage.upload(src, location)
}

func (storage *AWSS3Storage) DeleteFile(location string) (error) {
	return storage.delete(location)
}

func (storage *AWSS3Storage) PutImage(src, location, sizes string) ([]string, error) {
	//logger.Infof("PutImages: %+v, %+v, %+v", src, location, sizes)
	var locations []string

	dst := path.Join(storage.temp, location)
	if _, err := os.Stat(path.Dir(dst)); err != nil {
		if err = os.MkdirAll(path.Dir(dst), 0755); err != nil {
			return locations, err
		}
	}
	urls := make(map[string]*AWSS3StorageItem)
	fi1, err := os.Stat(src)
	if err != nil {
		return locations, err
	}
	if fi2, err := os.Stat(dst + ".json"); err != nil || !fi1.ModTime().Equal(fi2.ModTime()) {
		//logger.Infof("case1: %+v", dst + ".json")
		// Upload original
		if url, err := storage.upload(src, location); err == nil {
			urls[location] = &AWSS3StorageItem{
				Created:  time.Now(),
				Url:      url,
				Size:     fi1.Size(),
				Modified: fi1.ModTime(),
			}
			locations = append(locations, storage.rw(url))
		} else {
			return locations, err
		}
	} else {
		//logger.Infof("case2: %+v", dst + ".json")
		if bts, err := ioutil.ReadFile(dst + ".json"); err == nil {
			var item *AWSS3StorageItem
			if err = json.Unmarshal(bts, &item); err == nil {
				//logger.Infof("item: %+v", item)
				locations = append(locations, storage.rw(item.Url))
			} else {
				logger.Warningf("%v", err)
			}
		}else{
			logger.Warningf("%v", err)
		}
	}
	if sizes != "" {
		var img image.Image
		ext := strings.ToLower(filepath.Ext(src))
		if p := path.Join(path.Dir(dst), "resize"); len(p) > 0 {
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
				return locations, err
			}
			var height int
			if height, err = strconv.Atoi(pair[1]); err != nil {
				return locations, err
			}
			filename := path.Base(src)
			filename = filename[:len(filename) - len(filepath.Ext(filename))]
			filename = fmt.Sprintf("%s_%dx%d%s", filename, width, height, filepath.Ext(src))
			dst2 := path.Join(path.Dir(dst), "resize", filename)
			if fi2, err := os.Stat(dst2 + ".json"); err != nil || !fi1.ModTime().Equal(fi2.ModTime()) {
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
				//logger.Infof("case3: %+v", dst2 + ".json")
				m := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
				out, err := os.Create(dst2)
				if err != nil {
					return locations, err
				}
				if ext == ".jpg" || ext == ".jpeg" {
					if err = jpeg.Encode(out, m, &jpeg.Options{Quality: storage.quality}); err != nil {
						return locations, err
					}
					out.Close()
				}else if ext == ".png" {
					if err = png.Encode(out, m); err != nil {
						return locations, err
					}
					out.Close()
				}

				if err = os.Chtimes(dst2, fi1.ModTime(), fi1.ModTime()); err != nil {
					logger.Warningf("%v", err)
				}
				//
				if url, err := storage.upload(dst2, path.Join(path.Dir(location), "resize", filename)); err == nil {
					urls[path.Join(path.Dir(location), "resize", filename)] = &AWSS3StorageItem{
						Created: time.Now(),
						Url: url,
						Size: fi1.Size(),
						Modified: fi1.ModTime(),
					}
					locations = append(locations, fmt.Sprintf("%s %dw", storage.rw(url), width))
					if err = os.Remove(dst2); err != nil {
						logger.Warningf("%v", err)
					}
				} else {
					return locations, err
				}
			}else{
				//logger.Infof("case4: %+v", dst2 + ".json")
				if bts, err := ioutil.ReadFile(dst2 + ".json"); err == nil {
					var item *AWSS3StorageItem
					if err = json.Unmarshal(bts, &item); err == nil {
						//logger.Infof("item: %+v", item)
						locations = append(locations, fmt.Sprintf("%s %dw", storage.rw(item.Url), width))
					} else {
						logger.Warningf("%v", err)
					}
				}else{
					logger.Warningf("%v", err)
				}
			}
		}
	}
	//logger.Infof("urls: %+v", len(urls))
	for key, value := range urls {
		//logger.Infof("%v: %+v", key, value)
		if bts, err := json.Marshal(value); err == nil {
			file := path.Join(storage.temp, key + ".json")
			if err = ioutil.WriteFile(file, bts, 0755); err == nil {
				if err = os.Chtimes(file, value.Modified, value.Modified); err != nil {
					logger.Warningf("%v", err)
				}
			}else{
				logger.Warningf("%v", err)
			}
		}
	}

	return locations, nil
}

func (storage *AWSS3Storage) DeleteImage(location, sizes string) error {
	var err error
	if err = storage.delete(location); err != nil {
		return err
	}
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
		filename = filename[:len(filename) - len(filepath.Ext(filename))]
		filename = fmt.Sprintf("%s_%dx%d%s", filename, width, height, filepath.Ext(location))
		if err = storage.delete(path.Join(path.Dir(location), "resize", filename)); err != nil {
			logger.Warningf("%+v", err)
		}
	}
	return storage.delete(location)
}

/*func (s3 *AWSS3Storage) Copy(src, dst string) error {
	suffix := strings.Replace(dst, path.Join(s3.dir, "hugo", "static"), "", 1)
	dst = strings.Replace(dst, path.Join(s3.dir, "hugo", "static"), path.Join(s3.dir, "temp", "s3"), 1)
	if _, err := os.Stat(path.Dir(dst)); err != nil {
		os.MkdirAll(path.Dir(dst), 0755)
	}
	logger.Infof("Copy: %+v, %+v", src, dst)
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
	err = os.Chtimes(dst, fi1.ModTime(), fi1.ModTime())
	if err != nil {
		return err
	}

	uploader := s3manager.NewUploader(s3.session)

	file, err := os.Open(dst)
	if err != nil {
		return err
	}

	up, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s3.Bucket),
		ACL:    aws.String("public-read"),
		Key:    aws.String(suffix),
		Body:   file,
	})
	if err != nil {
		return err
	}

	logger.Infof("s3 up: %+v", up)

	return nil
}

func (s3 *AWSS3Storage) ImageResize(src, sizes string) ([]Image, error) {
	src = strings.Replace(src, path.Join(s3.dir, "hugo", "static"), path.Join(s3.dir, "temp", "s3"), 1)
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
			if err = jpeg.Encode(out, m, &jpeg.Options{Quality: s3.quality}); err != nil {
				return images, err
			}
			out.Close()
			os.Chtimes(path.Join(path.Dir(src), "resize", filename), fi1.ModTime(), fi1.ModTime())
		}
		images = append(images, Image{Filename: filename, Size: fmt.Sprintf("%dw", width)})
	}
	return images, nil
}*/
