package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// QueueName is the single durable queue match ingestion jobs flow through.
const QueueName = "match_ingestion"

// Queue wraps a RabbitMQ connection/channel dedicated to match ingestion.
type Queue struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// Dial connects to RabbitMQ and declares the durable ingestion queue.
func Dial(url string) (*Queue, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("ingestion: dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("ingestion: open channel: %w", err)
	}

	if _, err := ch.QueueDeclare(QueueName, true, false, false, false, nil); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("ingestion: declare queue: %w", err)
	}

	return &Queue{conn: conn, ch: ch}, nil
}

func (q *Queue) Close() {
	q.ch.Close()
	q.conn.Close()
}

// Publish enqueues a match ingestion job. It does not wait for the job to
// be processed - callers (the profile endpoint) get an instant response
// while ingestion happens in the background.
func (q *Queue) Publish(ctx context.Context, job MatchJob) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("ingestion: marshal job: %w", err)
	}

	err = q.ch.PublishWithContext(ctx, "", QueueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("ingestion: publish job: %w", err)
	}
	return nil
}

// Handler processes one match ingestion job. Returning an error causes the
// job to be requeued for retry.
type Handler func(ctx context.Context, job MatchJob) error

// Consume starts a pool of workerCount goroutines pulling jobs off the
// queue and running handle on each, until ctx is canceled. It blocks until
// all workers have returned.
func (q *Queue) Consume(ctx context.Context, workerCount int, handle Handler) error {
	if err := q.ch.Qos(workerCount, 0, false); err != nil {
		return fmt.Errorf("ingestion: set QoS: %w", err)
	}

	deliveries, err := q.ch.Consume(QueueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("ingestion: start consuming: %w", err)
	}

	done := make(chan struct{})
	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			defer func() { done <- struct{}{} }()
			runWorker(ctx, workerID, deliveries, handle)
		}(i)
	}
	for i := 0; i < workerCount; i++ {
		<-done
	}
	return nil
}

func runWorker(ctx context.Context, workerID int, deliveries <-chan amqp.Delivery, handle Handler) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}

			var job MatchJob
			if err := json.Unmarshal(d.Body, &job); err != nil {
				log.Printf("ingestion worker %d: dropping malformed job: %v", workerID, err)
				d.Nack(false, false)
				continue
			}

			if err := handle(ctx, job); err != nil {
				log.Printf("ingestion worker %d: job %+v failed, requeuing: %v", workerID, job, err)
				d.Nack(false, true)
				continue
			}

			d.Ack(false)
		}
	}
}
