package service

type Services interface {
	Share() ShareService
}

type services struct {
	shareService ShareService
}

func NewServices() Services {
	uploadService := newUploadService()
	configService := newConfigService()
	shareService := newShareService(configService, uploadService)
	return &services{
		shareService: shareService,
	}
}

func (s services) Share() ShareService {
	return s.shareService
}
