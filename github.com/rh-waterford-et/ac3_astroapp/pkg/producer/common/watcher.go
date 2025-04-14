package common

// Watcher monitors directories and triggers processing
type Watcher struct {
	Producer ProducerInterface
}

func NewWatcher(producer ProducerInterface) *Watcher {
	return &Watcher{Producer: producer}
}

func (w *Watcher) Watch() {
	w.Producer.ReadFiles()
}
