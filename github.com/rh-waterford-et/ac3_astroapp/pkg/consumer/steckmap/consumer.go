package steckmap

import "github.com/rh-waterford-et/ac3_astroapp/pkg/consumer/common"

type SteckmapConsumer struct {
	*common.BaseConsumer
}

func NewSteckmapConsumer() *SteckmapConsumer {
	base := common.NewBaseConsumer("steckmap")
	return &SteckmapConsumer{BaseConsumer: base}
}
