package ingestion

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// requires a real RabbitMQ reachable at INGESTION_TEST_AMQP_URL; skipped
// otherwise so `go test ./...` stays hermetic in CI without a broker.
func testQueue(t *testing.T) *Queue {
	t.Helper()
	url := os.Getenv("INGESTION_TEST_AMQP_URL")
	if url == "" {
		t.Skip("INGESTION_TEST_AMQP_URL not set, skipping RabbitMQ integration test")
	}

	q, err := Dial(url)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(q.Close)

	// Drain any leftovers from a previous run so counts in this test are exact.
	for {
		msg, ok, err := q.ch.Get(QueueName, true)
		if err != nil || !ok {
			break
		}
		_ = msg
	}

	return q
}

func TestPublishConsume_RoundTrip(t *testing.T) {
	q := testQueue(t)

	want := MatchJob{Region: "americas", MatchID: "NA1_123"}
	if err := q.Publish(context.Background(), want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got := make(chan MatchJob, 1)
	go q.Consume(ctx, 1, func(ctx context.Context, job MatchJob) error {
		got <- job
		cancel()
		return nil
	})

	select {
	case job := <-got:
		if job != want {
			t.Fatalf("got job %+v, want %+v", job, want)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("timed out waiting for the published job to be consumed")
	}
}

func TestConsume_DistributesAcrossWorkerPool(t *testing.T) {
	q := testQueue(t)

	const jobCount = 20
	for i := 0; i < jobCount; i++ {
		if err := q.Publish(context.Background(), MatchJob{Region: "americas", MatchID: "NA1_x"}); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var processed int32
	const workerCount = 4
	done := make(chan struct{})
	go func() {
		q.Consume(ctx, workerCount, func(ctx context.Context, job MatchJob) error {
			n := atomic.AddInt32(&processed, 1)
			if n == jobCount {
				cancel()
			}
			return nil
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("timed out waiting for consumers to drain the queue")
	}

	if got := atomic.LoadInt32(&processed); got != jobCount {
		t.Fatalf("processed %d jobs, want %d", got, jobCount)
	}
}

func TestConsume_RequeuesOnHandlerError(t *testing.T) {
	q := testQueue(t)

	if err := q.Publish(context.Background(), MatchJob{Region: "americas", MatchID: "NA1_retry"}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	var attempts int32
	go q.Consume(ctx, 1, func(ctx context.Context, job MatchJob) error {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			return errUnhandled
		}
		cancel()
		return nil
	})

	<-ctx.Done()
	if got := atomic.LoadInt32(&attempts); got < 2 {
		t.Fatalf("expected the job to be retried after a handler error, got %d attempt(s)", got)
	}
}

var errUnhandled = &testError{"transient failure"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
