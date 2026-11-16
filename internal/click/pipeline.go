package click

import (
	"context"
	"runtime"
	"sync"
	"time"
)

var _ Ingester = (*Pipeline)(nil)

type Metrics interface {
	ObserveClickDropped()
	SetQueueDepth(n int)
}

type BatchSink interface {
	Add(batch []Click)
}

type Pipeline struct { /* your fields */
	queueSize     int
	workers       int
	batchSize     int
	flushInterval time.Duration
	metrics       Metrics
	sink          BatchSink
	store         Store
	queue         chan Click
	mu            sync.RWMutex
	started       bool
	closed        bool
	wg            sync.WaitGroup
}

type PipelineOption func(*Pipeline)

func WithQueueSize(n int) PipelineOption {
	return func(p *Pipeline) {
		if n > 0 {
			p.queueSize = n
		}
	}
}
func WithWorkers(n int) PipelineOption {
	return func(p *Pipeline) {
		if n > 0 {
			p.workers = n
		}
	}
}
func WithBatchSize(n int) PipelineOption {
	return func(p *Pipeline) {
		if n > 0 {
			p.batchSize = n
		}
	}
}
func WithFlushInterval(d time.Duration) PipelineOption {
	return func(p *Pipeline) {
		if d > 0 {
			p.flushInterval = d
		}
	}
}

func WithMetrics(m Metrics) PipelineOption {
	return func(p *Pipeline) {
		if m != nil {
			p.metrics = m
		}
	}
}
func WithSink(s BatchSink) PipelineOption {
	return func(p *Pipeline) {
		if s != nil {
			p.sink = s
		}
	}
}

// A value that is below 1 (or not positive, for the interval), and a nil Metrics, are ignored: the default stays.
func NewPipeline(store Store, opts ...PipelineOption) *Pipeline {
	pipeline := &Pipeline{
		store:         store,
		queueSize:     10000,
		workers:       runtime.GOMAXPROCS(0),
		batchSize:     500,
		flushInterval: time.Second,
	}

	for _, o := range opts {
		o(pipeline)
	}

	pipeline.queue = make(chan Click, pipeline.queueSize)

	return pipeline
}

func (p *Pipeline) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started || p.closed {
		return
	}

	p.started = true
	p.wg.Add(p.workers)

	for i := 0; i < p.workers; i++ {
		go func() {
			defer p.wg.Done()
			p.runWorker()
		}()
	}

}
func (p *Pipeline) Submit(c Click) bool {
	p.queue <- c
}

func (p *Pipeline) Shutdown(ctx context.Context) error
