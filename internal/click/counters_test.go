package click

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestCountersUTCBucketsAndPartialDayWindow(t *testing.T) {
	c := NewCounters()
	t.Cleanup(c.Close)
	zone := time.FixedZone("UTC+2", 2*60*60)
	previousHour := time.Date(2030, 1, 2, 23, 0, 0, 0, time.UTC)
	midnight := previousHour.Add(time.Hour)
	c.Add([]Click{
		{Code: "a", At: time.Date(2030, 1, 3, 1, 15, 0, 0, zone)},
		{Code: "a", At: previousHour.Add(45 * time.Minute)},
		{Code: "a", At: midnight.Add(5 * time.Minute)},
	})

	ctx := context.Background()
	w := Window{From: previousHour, To: midnight.Add(time.Hour)}
	hours, err := c.Series(ctx, "a", w, GranularityHour)
	if err != nil {
		t.Fatal(err)
	}
	wantHours := []Bucket{{Start: previousHour, Clicks: 2}, {Start: midnight, Clicks: 1}}
	if !slices.Equal(hours, wantHours) {
		t.Fatalf("hours = %v, want %v", hours, wantHours)
	}

	days, err := c.Series(ctx, "a", w, GranularityDay)
	if err != nil {
		t.Fatal(err)
	}
	wantDays := []Bucket{
		{Start: midnight.Add(-24 * time.Hour), Clicks: 2},
		{Start: midnight, Clicks: 1},
	}
	if !slices.Equal(days, wantDays) {
		t.Fatalf("days = %v, want %v", days, wantDays)
	}

	w.From = previousHour.Add(30 * time.Minute)
	days, err = c.Series(ctx, "a", w, GranularityDay)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(days, wantDays[1:]) {
		t.Errorf("partial window days = %v, want %v", days, wantDays[1:])
	}
	top, err := c.Top(ctx, w, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(top, []Total{{Code: "a", Clicks: 1}}) {
		t.Errorf("partial window top = %v, want a=1", top)
	}
}

func TestCountersValidationOrder(t *testing.T) {
	c := NewCounters()
	c.Close()
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name string
		ctx  context.Context
		g    Granularity
		n    int
		want error
	}{
		{name: "context before granularity and closed", ctx: cancelled, g: "week", n: 1, want: context.Canceled},
		{name: "context before n and closed", ctx: cancelled, g: GranularityHour, n: 0, want: context.Canceled},
		{name: "granularity before closed", ctx: context.Background(), g: "week", n: 1, want: ErrInvalidGranularity},
		{name: "n before closed", ctx: context.Background(), g: GranularityHour, n: 0, want: ErrInvalidN},
		{name: "closed after valid arguments", ctx: context.Background(), g: GranularityHour, n: 1, want: ErrClosed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.n != 0 {
				if _, err := c.Series(tt.ctx, "a", Window{}, tt.g); !errors.Is(err, tt.want) {
					t.Errorf("Series error = %v, want %v", err, tt.want)
				}
			}
			if tt.g == GranularityHour {
				if _, err := c.Top(tt.ctx, Window{}, tt.n); !errors.Is(err, tt.want) {
					t.Errorf("Top error = %v, want %v", err, tt.want)
				}
			}
		})
	}
}

func TestCountersOwnInputAndReturnIndependentResults(t *testing.T) {
	c := NewCounters()
	t.Cleanup(c.Close)
	at := time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC)
	batch := []Click{{Code: "a", At: at}, {Code: "a", At: at}}
	c.Add(batch)
	batch[0] = Click{Code: "changed", At: at.Add(time.Hour)}
	batch[1] = Click{}

	ctx := context.Background()
	w := Window{From: at, To: at.Add(2 * time.Hour)}
	wantBuckets := []Bucket{{Start: at, Clicks: 2}}
	wantTotals := []Total{{Code: "a", Clicks: 2}}
	for i := 0; i < 2; i++ {
		buckets, err := c.Series(ctx, "a", w, GranularityHour)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(buckets, wantBuckets) {
			t.Fatalf("Series = %v, want %v", buckets, wantBuckets)
		}
		totals, err := c.Top(ctx, w, 10)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(totals, wantTotals) {
			t.Fatalf("Top = %v, want %v", totals, wantTotals)
		}
		buckets[0] = Bucket{}
		totals[0] = Total{}
	}
}

func TestCountersCloseDuringConcurrentOperations(t *testing.T) {
	c := NewCounters()
	t.Cleanup(c.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	at := time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC)
	w := Window{From: at, To: at.Add(time.Hour)}
	start := make(chan struct{})
	var ready, workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		ready.Add(1)
		workers.Add(1)
		go func() {
			defer workers.Done()
			c.Add([]Click{{Code: "a", At: at}})
			ready.Done()
			<-start
			for j := 0; j < 100; j++ {
				c.Add([]Click{{Code: "a", At: at}})
				if _, err := c.Series(ctx, "a", w, GranularityHour); err != nil && !errors.Is(err, ErrClosed) {
					t.Error(err)
					return
				}
				if _, err := c.Top(ctx, w, 1); err != nil && !errors.Is(err, ErrClosed) {
					t.Error(err)
					return
				}
			}
		}()
	}
	ready.Wait()
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			c.Close()
		}()
	}
	close(start)
	finished := make(chan struct{})
	go func() {
		workers.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-ctx.Done():
		t.Fatal("concurrent operations did not finish:", ctx.Err())
	}
}
