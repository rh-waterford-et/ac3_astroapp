package ppfx

import (
	"github.com/rh-waterford-et/ac3_astroapp/pkg/producer/common"
)

type PPFXProducer struct {
	*common.BaseProducer
}

func NewPPFXProducer(batchSize int, inputDir, outputDir string, eventQueue chan common.Event) *PPFXProducer {
	base := common.NewBaseProducer(batchSize, inputDir, outputDir, eventQueue)
	return &PPFXProducer{
		BaseProducer: base,
	}
}

func (p *PPFXProducer) AddFile(file common.DataFile) {
	p.BaseProducer.AddFile(file)
}

func (p *PPFXProducer) SendBatch() {
	p.BaseProducer.SendBatch()
}
