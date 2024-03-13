package service

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
	"net/http"
	"os"
)

const (
	maxPartSize = int64(1024 * 1024)
	maxRetries  = 3
)

type ShareService interface {
	Share(filename string) error
}

type shareService struct {
	configService ConfigService
}

func newShareService(configService ConfigService) ShareService {
	return &shareService{
		configService: configService,
	}
}

func (s shareService) Share(filename string) error {
	if err := s.configService.Setup(); err != nil {
		return err
	}

	if filename == "" {
		return fmt.Errorf("no filename provided")
	}
	_, err := os.Stat(filename)
	if err != nil {
		return fmt.Errorf("file does not exist")
	}

	cfg := s.configService.Get()

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	size := fileInfo.Size()
	buffer := make([]byte, size)
	fileType := http.DetectContentType(buffer)

	storage := s3.New(sess)
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(cfg.BucketName),
		Key:         aws.String(filename),
		ContentType: aws.String(fileType),
	}
	resp, err := storage.CreateMultipartUpload(input)
	if err != nil {
		return fmt.Errorf("cannot create multipart upload: %w", err)
	}

	var curr, partLength int64
	var remaining = size
	var completedParts []*s3.CompletedPart
	partNumber := 1

	bar := progressbar.DefaultBytes(size, "sharing")
	for curr = 0; remaining != 0; curr += partLength {
		_ = bar.Add(int(partLength))
		if remaining < maxPartSize {
			partLength = remaining
		} else {
			partLength = maxPartSize
		}
		completedPart, err := uploadPart(storage, resp, buffer[curr:curr+partLength], partNumber)
		if err != nil {
			logrus.Error(err)
			err := abortMultipartUpload(storage, resp)
			if err != nil {
				logrus.Error(err)
			}
			return err
		}
		remaining -= partLength
		partNumber++
		completedParts = append(completedParts, completedPart)
	}
	bar.Reset()

	completeResponse, err := completeMultipartUpload(storage, resp, completedParts)
	if err != nil {
		return err
	}
	_ = completeResponse
	logrus.Infof("File uploaded successfully. File location: ")

	return err
}

func completeMultipartUpload(svc *s3.S3, resp *s3.CreateMultipartUploadOutput, completedParts []*s3.CompletedPart) (*s3.CompleteMultipartUploadOutput, error) {
	completeInput := &s3.CompleteMultipartUploadInput{
		Bucket:   resp.Bucket,
		Key:      resp.Key,
		UploadId: resp.UploadId,
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	return svc.CompleteMultipartUpload(completeInput)
}

func uploadPart(svc *s3.S3, resp *s3.CreateMultipartUploadOutput, fileBytes []byte, partNumber int) (*s3.CompletedPart, error) {
	tryNum := 1
	partInput := &s3.UploadPartInput{
		Body:          bytes.NewReader(fileBytes),
		Bucket:        resp.Bucket,
		Key:           resp.Key,
		PartNumber:    aws.Int64(int64(partNumber)),
		UploadId:      resp.UploadId,
		ContentLength: aws.Int64(int64(len(fileBytes))),
	}

	for tryNum <= maxRetries {
		uploadResult, err := svc.UploadPart(partInput)
		if err != nil {
			if tryNum == maxRetries {
				var awsErr awserr.Error
				if errors.As(err, &awsErr) {
					return nil, awsErr
				}
				return nil, err
			}
			tryNum++
		} else {
			return &s3.CompletedPart{
				ETag:       uploadResult.ETag,
				PartNumber: aws.Int64(int64(partNumber)),
			}, nil
		}
	}
	return nil, nil
}

func abortMultipartUpload(svc *s3.S3, resp *s3.CreateMultipartUploadOutput) error {
	abortInput := &s3.AbortMultipartUploadInput{
		Bucket:   resp.Bucket,
		Key:      resp.Key,
		UploadId: resp.UploadId,
	}
	_, err := svc.AbortMultipartUpload(abortInput)
	return err
}
