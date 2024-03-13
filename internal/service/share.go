package service

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.design/x/clipboard"
	"os"
	"strings"
)

type ShareService interface {
	Share(filename string, copyToClipboard bool) error
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

func (s shareService) Share(filename string, copyToClipboard bool) error {
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
	logrus.Infof("file uploaded successfully")
	err = s.uploadService.GrantPublicAccess(key, cfg)
	if err != nil {
		return err
	}

	publicUrl := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", strings.ToLower(cfg.BucketName), key)
	logrus.Infof("file shared: %s", publicUrl)

	if copyToClipboard {
		err = clipboard.Init()
		if err != nil {
			logrus.Panicf("unable to init clipboard: %w", err)
		}
		clipboard.Write(clipboard.FmtText, []byte(publicUrl))
		logrus.Infof("copied to clipboard")
	} else {
		logrus.Infof("tip: add \"-c\" before filename to copy public url to clipboard")
	}

	return nil
}
