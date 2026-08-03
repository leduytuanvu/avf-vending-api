package payments

import "github.com/avf/avf-vending-api/internal/config"

func registerLivePSPAdapters(r *Registry, cfg *config.Config) {
	if r == nil || cfg == nil {
		return
	}
	r.Register(NewMoMoProvider(cfg))
	r.Register(NewZaloPayProvider(cfg, false)) // key zalopay
	r.Register(NewZaloPayProvider(cfg, true))  // key vietqr
	r.Register(NewVNPayProvider(cfg))
	r.Register(NewShopeePayProvider(cfg))
}
