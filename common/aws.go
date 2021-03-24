package common

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"os"
)

/*var AccessKeyID = "AKIAYD5A7GLUACXWFKPM"
var SecretAccessKey = "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"
var MyRegion = "eu-central-1"
var MyBucket = "moebelhaus"*/

func GetS3Session(accessKeyID, secretAccessKey, region string) (*session.Session, error) {
	//AccessKeyID = GetEnvWithKey("AWS_ACCESS_KEY_ID")
	//SecretAccessKey = GetEnvWithKey("AWS_SECRET_ACCESS_KEY")
	//MyRegion = "eu-central-1"
	sess, err := session.NewSession(
		&aws.Config{
			Region: aws.String(region),
			Credentials: credentials.NewStaticCredentials(
				accessKeyID,
				secretAccessKey,
				"", // a token will be created when the session it's used.
			),
		})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func PostS3File(s *session.Session, bucket, src, dst string) (*s3manager.UploadOutput, error) {
	uploader := s3manager.NewUploader(s)

	file, err := os.Open(src)
	if err != nil {
		return nil, err
	}

	up, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		ACL:    aws.String("public-read"),
		Key:    aws.String(dst),
		Body:   file,
	})
	if err != nil {
		return nil, err
	}

	return up, nil
}

