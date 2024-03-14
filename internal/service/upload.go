package service

import (
	"bytes"
	"context"
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
	"share/internal/dto"
)

const (
	maxPartSize              = int64(1024 * 1024)
	maxRetries               = 3
	showProgressBarThreshold = int64(5 * 1024 * 1024)
)

type UploadService interface {
	Upload(filename string, cfg dto.Config) (string, error)
	GrantPublicAccess(key string, cfg dto.Config) error
}

type uploadService struct {
}

func newUploadService() UploadService {
	return &uploadService{}
}

func (u uploadService) Upload(filename string, cfg dto.Config) (string, error) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("cannot open file: %w", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	size := fileInfo.Size()
	buffer := make([]byte, size)
	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}
	fileType := http.DetectContentType(buffer)

	storage := s3.New(sess)
	input := &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(cfg.BucketName),
		Key:         aws.String(filename),
		ContentType: aws.String(fileType),
	}
	resp, err := storage.CreateMultipartUpload(input)
	if err != nil {
		return "", fmt.Errorf("cannot create multipart upload: %w", err)
	}

	var curr, partLength int64
	var remaining = size
	var completedParts []*s3.CompletedPart
	partNumber := 1

	showProgressBar := size > showProgressBarThreshold

	var bar *progressbar.ProgressBar
	if showProgressBar {
		bar = progressbar.DefaultBytes(size, "sharing")
	}
	for curr = 0; remaining != 0; curr += partLength {
		if showProgressBar {
			_ = bar.Add(int(partLength))
		}
		if remaining < maxPartSize {
			partLength = remaining
		} else {
			partLength = maxPartSize
		}
		completedPart, err := u.uploadPart(storage, resp, buffer[curr:curr+partLength], partNumber)
		if err != nil {
			logrus.Error(err)
			err := u.abortMultipartUpload(storage, resp)
			if err != nil {
				logrus.Error(err)
			}
			return "", err
		}
		remaining -= partLength
		partNumber++
		completedParts = append(completedParts, completedPart)
	}
	if showProgressBar {
		bar.Reset()
	}

	completeResponse, err := u.completeMultipartUpload(storage, resp, completedParts)
	if err != nil {
		return "", err
	}
	if completeResponse.Key == nil {
		return "", fmt.Errorf("unable to get object key")
	}
	return *completeResponse.Key, nil
}

func (u uploadService) GrantPublicAccess(key string, cfg dto.Config) error {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	storage := s3.New(sess)

	aclPolicy := &s3.PutObjectAclInput{
		Bucket: aws.String(cfg.BucketName),
		Key:    aws.String(key),
		ACL:    aws.String("public-read"),
	}
	_, err := storage.PutObjectAclWithContext(context.Background(), aclPolicy)
	if err != nil {
		return fmt.Errorf("error granting public read access\nhint: make sure your bucket has ACLs enabled and does not \"Block public access to buckets and objects granted through new access control lists\"\nSee: https://s3.console.aws.amazon.com/s3/buckets/" + cfg.BucketName + "?tab=permissions")
	}
	logrus.Infof("public-read ACL for %s set", key)
	return nil
}

func (uploadService) completeMultipartUpload(svc *s3.S3, resp *s3.CreateMultipartUploadOutput, completedParts []*s3.CompletedPart) (*s3.CompleteMultipartUploadOutput, error) {
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

func (uploadService) uploadPart(svc *s3.S3, resp *s3.CreateMultipartUploadOutput, fileBytes []byte, partNumber int) (*s3.CompletedPart, error) {
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

func (uploadService) abortMultipartUpload(svc *s3.S3, resp *s3.CreateMultipartUploadOutput) error {
	abortInput := &s3.AbortMultipartUploadInput{
		Bucket:   resp.Bucket,
		Key:      resp.Key,
		UploadId: resp.UploadId,
	}
	_, err := svc.AbortMultipartUpload(abortInput)
	return err
}
