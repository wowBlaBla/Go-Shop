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
	"math/rand"
	"net/url"
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

func NewAWSS3Storage(accessKeyID, secretAccessKey, region, bucket, prefix string, temp string, resize bool, quality int, cdn string, rewrite string) (*AWSS3Storage, error){
	storage := &AWSS3Storage{
		prefix: prefix,
		resize: resize,
		quality: quality,
		rewrite: rewrite,
		cdn: cdn,
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
	storage.Open()
	//
	return storage, err
}

type AWSS3Storage struct {
	session *session.Session
	AccessKeyID string
	SecretAccessKey string
	Region string
	Bucket string
	Database map[string]*AWSS3StorageItem
	//
	prefix string
	temp string // full/path
	resize bool
	quality int
	cdn string
	rewrite string
}

func (storage *AWSS3Storage) Open() error {
	database := path.Join(storage.temp, "database.json")
	storage.Database = make(map[string]*AWSS3StorageItem)
	if _, err := os.Stat(database); err == nil {
		if bts, err := ioutil.ReadFile(database); err == nil {
			if err = json.Unmarshal(bts, &storage.Database); err != nil {
				storage.Database = make(map[string]*AWSS3StorageItem)
				return err
			}
		}else{
			return err
		}
	}
	return nil
}

func (storage *AWSS3Storage) Save() error {
	database := path.Join(storage.temp, "database.json")
	if bts, err := json.Marshal(storage.Database); err == nil {
		return ioutil.WriteFile(database, bts, 0755)
	} else {
		return err
	}
}

func (storage *AWSS3Storage) Close() error {
	return storage.Save()
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
	if u, err := url.Parse(link); err == nil {
		if storage.cdn != "" {
			u.Host = storage.cdn
		}
		if storage.rewrite != "" {
			u.Path = storage.rewrite + u.Path
		}
		return u.String()
	}
	return link
}

func (storage *AWSS3Storage) PutFile(src, location string) (string, error) {
	dst := path.Join(storage.temp, location)
	if _, err := os.Stat(path.Dir(dst)); err != nil {
		if err = os.MkdirAll(path.Dir(dst), 0755); err != nil {
			return location, err
		}
	}
	urls := make(map[string]*AWSS3StorageItem)
	fi1, err := os.Stat(src)
	if err != nil {
		return location, err
	}
	// //
	if item, found := storage.Database[location]; !found || fi1.ModTime().Sub(item.Modified) > time.Second {
		if url, err := storage.upload(src, location); err == nil {
			urls[location] = &AWSS3StorageItem{
				Created:  time.Now(),
				Url:      url,
				Size:     fi1.Size(),
				Modified: fi1.ModTime(),
			}
			location = storage.rw(fmt.Sprintf("%s?%s", url, strconv.FormatInt(time.Now().Unix(), 36)))
		} else {
			return location, err
		}
	}else{
		location = storage.rw(item.Url)
	}

	for key, value := range urls {
		storage.Database[key] = value
	}

	return location, err
}

func (storage *AWSS3Storage) DeleteFile(location string) (error) {
	delete(storage.Database, location)
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
	if item, found := storage.Database[location]; !found || fi1.ModTime().Sub(item.Modified) > time.Second {
		if url, err := storage.upload(src, location); err == nil {
			urls[location] = &AWSS3StorageItem{
				Created:  time.Now(),
				Url:      url,
				Size:     fi1.Size(),
				Modified: fi1.ModTime(),
			}
			locations = append(locations, storage.rw(fmt.Sprintf("%s?%s", url, strconv.FormatInt(time.Now().Unix(), 36))))
		} else {
			return locations, err
		}
	}else{
		locations = append(locations, storage.rw(item.Url))
	}
	if sizes != "" {
		if storage.resize {
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
				filename := path.Base(location)
				filename = filename[:len(filename) - len(filepath.Ext(filename))]
				filename = fmt.Sprintf("%s_%d_%dx%d%s", filename, storage.quality, width, height, filepath.Ext(src))
				dst2 := path.Join(path.Dir(dst), "resize", filename)
				// //
				key := path.Join(path.Dir(location), "resize", filename)
				if item, found := storage.Database[key]; !found || fi1.ModTime().Sub(item.Modified) > time.Second {
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
						locations = append(locations, fmt.Sprintf("%s?%s %dw", storage.rw(url), strconv.FormatInt(time.Now().Unix(), 36), width))
						if err = os.Remove(dst2); err != nil {
							logger.Warningf("%v", err)
						}
					} else {
						return locations, err
					}
				}else{
					locations = append(locations, fmt.Sprintf("%s?%s %dw", storage.rw(item.Url), strconv.FormatInt(time.Now().Unix(), 36), width))
				}
			}
		}else{
			if len(locations) > 0 {
				origin := locations[0]
				for _, size := range strings.Split(sizes, ",") {
					pair := strings.Split(size, "x")
					var width int
					if width, err = strconv.Atoi(pair[0]); err != nil {
						return locations, err
					}
					locations = append(locations, fmt.Sprintf("%v?w=%v&q=%v", origin, width, storage.quality))
				}
			}
		}
	}
	for key, value := range urls {
		storage.Database[key] = value
	}
	if rand.Intn(10) == 0 {
		if err = storage.Save(); err != nil {
			logger.Warningf("%+v", err)
		}
	}
	return locations, nil
}

func (storage *AWSS3Storage) DeleteImage(location, sizes string) error {
	var err error
	delete(storage.Database, location)
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
		delete(storage.Database, path.Join(path.Dir(location), "resize", filename))
		if err = storage.delete(path.Join(path.Dir(location), "resize", filename)); err != nil {
			logger.Warningf("%+v", err)
		}
	}
	return storage.delete(location)
}
