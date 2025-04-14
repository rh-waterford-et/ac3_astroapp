package steckmap

import (
	"github.com/rh-waterford-et/ac3_astroapp/pkg/producer/common"
)

type SteckmapProducer struct {
	*common.BaseProducer
}

func NewSteckmapProducer(batchSize int, inputDir, outputDir string, eventQueue chan common.Event) *SteckmapProducer {
	base := common.NewBaseProducer(batchSize, inputDir, outputDir, eventQueue)
	return &SteckmapProducer{
		BaseProducer: base,
	}
}

func (p *SteckmapProducer) AddFile(file common.DataFile) {
	p.BaseProducer.AddFile(file)
}

func (p *SteckmapProducer) SendBatch() {
	p.BaseProducer.SendBatch()
}
