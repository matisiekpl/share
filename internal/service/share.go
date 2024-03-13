package service

import (
	"fmt"
	"github.com/sirupsen/logrus"
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
	uploadService UploadService
}

func newShareService(configService ConfigService, uploadService UploadService) ShareService {
	return &shareService{
		configService: configService,
		uploadService: uploadService,
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
	key, err := s.uploadService.Upload(filename, cfg)
	if err != nil {
		return err
	}
	logrus.Infof("File uploaded successfully. File location: %s", key)
	return err
}
