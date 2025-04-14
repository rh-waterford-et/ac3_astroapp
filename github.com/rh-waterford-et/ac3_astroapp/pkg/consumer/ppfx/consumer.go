package ppfx

import "github.com/rh-waterford-et/ac3_astroapp/pkg/consumer/common"

type PPFXConsumer struct {
	*common.BaseConsumer
}

func NewPPFXConsumer() *PPFXConsumer {
	base := common.NewBaseConsumer("ppfx")
	return &PPFXConsumer{BaseConsumer: base}
}
