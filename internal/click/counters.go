package click

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/al-bo/rd-linkforge/pkg/topk"
)

type counterOperation uint8

const (
	counterAdd counterOperation = iota
	counterSeries
	counterTop
)

type counterRequest struct {
	operation   counterOperation
	batch       []Click
	code        string
	window      Window
	granularity Granularity
	n           int
	reply       chan counterResponse
}

type counterResponse struct {
	buckets []Bucket
	totals  []Total
	err     error
}

// Counters stores hourly click counts and is safe for concurrent use.
// Use NewCounters to initialize it and Close to stop its owner goroutine.
type Counters struct {
	requests  chan counterRequest
	done      chan struct{}
	stopped   chan struct{}
	closeOnce sync.Once
}

var _ Aggregator = (*Counters)(nil)
var _ BatchSink = (*Counters)(nil)

var (
	ErrInvalidGranularity = errors.New("click: invalid granularity")
	ErrInvalidN           = errors.New("click: n must be positive")
	ErrClosed             = errors.New("click: counters are closed")
)

// NewCounters starts the owner goroutine.
func NewCounters() *Counters {
	c := &Counters{
		requests: make(chan counterRequest),
		done:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}
	go c.run()
	return c
}

// Add copies the batch and returns when the owner goroutine has taken the copy.
// A click with an empty Code is ignored. After Close it does nothing and never blocks.
func (c *Counters) Add(batch []Click) {
	select {
	case <-c.done:
		return
	default:
	}

	request := counterRequest{
		operation: counterAdd,
		batch:     slices.Clone(batch),
	}

	select {
	case c.requests <- request:
	case <-c.done:
	}
}

// Series returns nonempty hourly or daily UTC buckets ordered by start time.
func (c *Counters) Series(ctx context.Context, code string, w Window, g Granularity) ([]Bucket, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if g != GranularityHour && g != GranularityDay {
		return nil, ErrInvalidGranularity
	}

	response, err := c.query(ctx, counterRequest{
		operation:   counterSeries,
		code:        code,
		window:      w,
		granularity: g,
	})
	return response.buckets, err
}

// Top returns up to n codes ordered by click count descending, then code ascending.
func (c *Counters) Top(ctx context.Context, w Window, n int) ([]Total, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if n < 1 {
		return nil, ErrInvalidN
	}

	response, err := c.query(ctx, counterRequest{
		operation: counterTop,
		window:    w,
		n:         n,
	})
	return response.totals, err
}

// Close stops the owner goroutine. It may be called more than once, and at the same time as the other methods.
func (c *Counters) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
	<-c.stopped
}

func (c *Counters) query(ctx context.Context, request counterRequest) (counterResponse, error) {
	select {
	case <-c.done:
		return counterResponse{}, ErrClosed
	default:
	}

	// The owner can reply even if the caller has stopped waiting.
	request.reply = make(chan counterResponse, 1)

	select {
	case c.requests <- request:
	case <-ctx.Done():
		return counterResponse{}, ctx.Err()
	case <-c.done:
		return counterResponse{}, ErrClosed
	}

	select {
	case response := <-request.reply:
		return response, response.err
	case <-ctx.Done():
		return counterResponse{}, ctx.Err()
	case <-c.done:
		return counterResponse{}, ErrClosed
	}
}

func (c *Counters) run() {
	defer close(c.stopped)
	stat := make(map[string]map[time.Time]int64)

	for {
		select {
		case <-c.done:
			return
		default:
		}

		select {
		case <-c.done:
			return
		case request := <-c.requests:
			switch request.operation {
			case counterAdd:
				for _, event := range request.batch {
					if event.Code == "" {
						continue
					}
					if stat[event.Code] == nil {
						stat[event.Code] = make(map[time.Time]int64)
					}
					hour := event.At.UTC().Truncate(time.Hour)
					stat[event.Code][hour]++
				}
			case counterSeries:
				buckets := seriesFromHours(stat[request.code], request.window, request.granularity)
				request.reply <- counterResponse{buckets: buckets}
			case counterTop:
				totals, err := topFromHours(stat, request.window, request.n)
				request.reply <- counterResponse{totals: totals, err: err}
			}
		}
	}
}

func seriesFromHours(hours map[time.Time]int64, w Window, g Granularity) []Bucket {
	grouped := make(map[time.Time]int64)
	for start, count := range hours {
		if start.Before(w.From) || !start.Before(w.To) {
			continue
		}
		if g == GranularityDay {
			start = start.Truncate(24 * time.Hour)
		}
		grouped[start] += count
	}

	buckets := make([]Bucket, 0, len(grouped))
	for start, count := range grouped {
		buckets = append(buckets, Bucket{Start: start, Clicks: count})
	}
	slices.SortFunc(buckets, func(a, b Bucket) int {
		return a.Start.Compare(b.Start)
	})
	return buckets
}

func topFromHours(stat map[string]map[time.Time]int64, w Window, n int) ([]Total, error) {
	ranking, err := topk.New(n)
	if err != nil {
		return nil, err
	}

	for code, hours := range stat {
		var total int64
		for start, count := range hours {
			if !start.Before(w.From) && start.Before(w.To) {
				total += count
			}
		}
		if total > 0 {
			ranking.Add(code, total)
		}
	}

	items := ranking.Result()
	totals := make([]Total, 0, len(items))
	for _, item := range items {
		totals = append(totals, Total{Code: item.Key, Clicks: item.Count})
	}
	return totals, nil
}
