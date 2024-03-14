package service

type Services interface {
	Share() ShareService
	Config() ConfigService
	Upload() UploadService
}

type services struct {
	shareService  ShareService
	configService ConfigService
	uploadService UploadService
}

func NewServices() Services {
	uploadService := newUploadService()
	configService := newConfigService()
	shareService := newShareService(configService, uploadService)
	return &services{
		shareService:  shareService,
		configService: configService,
		uploadService: uploadService,
	}
}

func (s services) Share() ShareService {
	return s.shareService
}

func (s services) Config() ConfigService {
	return s.configService
}

func (s services) Upload() UploadService {
	return s.uploadService
}
