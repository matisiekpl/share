package service

type Services interface {
	Share() ShareService
}

type services struct {
	shareService ShareService
}

func NewServices() Services {
	configService := newConfigService()
	shareService := newShareService(configService)
	return &services{
		shareService: shareService,
	}
}

func (s services) Share() ShareService {
	return s.shareService
}
