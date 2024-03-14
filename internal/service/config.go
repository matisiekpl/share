package service

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/manifoldco/promptui"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
	"share/internal/dto"
	"strings"
)

type ConfigService interface {
	Setup(force bool, skipAsking bool) error
	Get() dto.Config
	ListBuckets() []string
}

type configService struct {
}

func newConfigService() ConfigService {
	return &configService{}
}

func (configService) ListBuckets() []string {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	result, err := s3.New(sess).ListBuckets(nil)
	if err != nil {
		logrus.Errorf(fmt.Errorf("error listing buckets: %w", err).Error())
		os.Exit(1)
	}
	var names []string
	for _, bucket := range result.Buckets {
		names = append(names, *bucket.Name)
	}
	return names
}

func (configService) Get() dto.Config {
	homedir, err := os.UserHomeDir()
	if err != nil {
		logrus.Fatal(err)
	}
	configFilename := fmt.Sprintf("%s/.share.yaml", homedir)
	f, err := os.Open(configFilename)
	if err != nil {
		logrus.Fatal(err)
	}
	defer f.Close()

	var cfg dto.Config
	err = yaml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		logrus.Fatal(err)
	}
	return cfg
}

func (c configService) Setup(force bool, skipAsking bool) error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot get user home directory: %w", err)
	}
	configFilename := fmt.Sprintf("%s/.share.yaml", homedir)
	_, err = os.Stat(configFilename)
	if err != nil || force {
		var configureNow string
		if !skipAsking {
			logrus.Info("share is not configured yet, do you want to configure it now?")
			// config file does not exist, create it
			configureNowPrompt := promptui.Prompt{
				Label: "Configure share now? [y/n]",
				Validate: func(s string) error {
					if strings.ToLower(s) != "y" && strings.ToLower(s) != "n" {
						return fmt.Errorf("invalid input")
					}
					return nil
				},
			}
			configureNow, err = configureNowPrompt.Run()
			if err != nil {
				return fmt.Errorf("prompt failed %w", err)
			}
		}
		if configureNow == "y" || skipAsking {
			logrus.Info("share will use AWS credentials from ~/.aws/credentials")
			configureNowPrompt := promptui.Select{
				Label: "Select AWS S3 bucket to store files",
				Items: c.ListBuckets(),
			}

			_, bucketName, err := configureNowPrompt.Run()
			if err != nil {
				return fmt.Errorf("prompt failed %w", err)
			}
			logrus.Infof("saving bucket name: %s to ~/.share.yaml", bucketName)

			// create config file
			f, err := os.Create(configFilename)
			if err != nil {
				return fmt.Errorf("cannot create config file: %w", err)
			}
			defer f.Close()

			marshaledYaml, err := yaml.Marshal(dto.Config{
				BucketName: bucketName,
			})
			if err != nil {
				return fmt.Errorf("cannot marshal config: %w", err)
			}
			_, err = f.WriteString(string(marshaledYaml))
			if err != nil {
				return fmt.Errorf("cannot write config file: %w", err)
			}
			logrus.Info("share is now configured")
		} else {
			os.Exit(0)
		}
	}
	return nil
}
