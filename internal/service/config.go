package service

import (
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
	"share/internal/dto"
	"strings"
)

type ConfigService interface {
	Setup() error
	Get() dto.Config
}

type configService struct {
}

func newConfigService() ConfigService {
	return &configService{}
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

func (configService) Setup() error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot get user home directory: %w", err)
	}
	configFilename := fmt.Sprintf("%s/.share.yaml", homedir)
	_, err = os.Stat(configFilename)
	if err != nil {
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
		configureNow, err := configureNowPrompt.Run()
		if err != nil {
			return fmt.Errorf("prompt failed %w", err)
		}
		if configureNow == "y" {
			logrus.Info("share will use AWS credentials from ~/.aws/credentials")
			configureNowPrompt := promptui.Prompt{
				Label: "Enter the name of the AWS S3 bucket name to store files",
				Validate: func(s string) error {
					if s == "" {
						return fmt.Errorf("invalid bucket name")
					}
					return nil
				},
			}
			bucketName, err := configureNowPrompt.Run()
			if err != nil {
				return fmt.Errorf("prompt failed %w", err)
			}
			logrus.Infof("Saving bucket name: %s to ~/.share.yaml", bucketName)

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
