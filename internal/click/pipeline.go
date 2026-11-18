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

type Pipeline struct {
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
	ctx           context.Context
	cancel        context.CancelFunc
	done          chan struct{}
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
	pipeline.ctx, pipeline.cancel = context.WithCancel(context.Background())
	pipeline.done = make(chan struct{})

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
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		if p.metrics != nil {
			p.metrics.ObserveClickDropped()
		}
		return false
	}

	select {
	case p.queue <- c:
		if p.metrics != nil {
			p.metrics.SetQueueDepth(len(p.queue))
		}
		return true

	default:
		if p.metrics != nil {
			p.metrics.ObserveClickDropped()
		}
		return false
	}
}

func (p *Pipeline) runWorker() {
	batch := make([]Click, 0, p.batchSize)
	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case c, ok := <-p.queue:
			if !ok {
				p.flush(batch)
				return
			}

			batch = append(batch, c)

			if len(batch) >= p.batchSize {
				p.flush(batch)
				batch = make([]Click, 0, p.batchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				p.flush(batch)
				batch = make([]Click, 0, p.batchSize)
			}
		}
	}
}

func (p *Pipeline) flush(batch []Click) {
	if len(batch) == 0 {
		return
	}

	if err := p.store.Append(p.ctx, batch); err != nil {
		if p.metrics != nil {
			for range batch {
				p.metrics.ObserveClickDropped()
			}
		}
		return
	}

	if p.sink != nil {
		p.sink.Add(batch)
	}
}

func (p *Pipeline) Shutdown(ctx context.Context) error {
	p.mu.Lock()

	if !p.closed {
		p.closed = true
		close(p.queue)

		if !p.started {
			p.cancel()
			close(p.done)
		} else {
			go func() {
				p.wg.Wait()
				p.cancel()
				close(p.done)
			}()
		}
	}

	p.mu.Unlock()

	select {
	case <-p.done:
		return nil
	default:
	}

	select {
	case <-p.done:
		return nil

	case <-ctx.Done():
		p.cancel()
		return ctx.Err()
	}
}
