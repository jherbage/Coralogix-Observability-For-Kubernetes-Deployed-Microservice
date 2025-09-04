package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/jherbage/Coralogix-Observability-For-Kubernetes-Deployed-Microservice/go/message"
	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const (
	rabbitMQURL = "amqp://admin:adminpassword@my-rabbitmq.demo.svc.cluster.local:5672/"
	queueName   = "work"
	serviceName = "producer-service"
)

func main() {
	// Initialize OpenTelemetry Tracer
	tp, err := initTracer()
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("failed to shut down tracer: %v", err)
		}
	}()

	tracer := otel.Tracer(serviceName)

	// Connect to RabbitMQ
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	// Publish messages to the queue indefinitely
	messageID := 1
	for {
		// Generate a random WaitTime between 1 and 20 seconds
		waitTime := rand.Intn(20) + 1

		// Create a new Message object
		msg := message.NewMessage(
			fmt.Sprintf("%d", messageID),          // ID
			fmt.Sprintf("Message #%d", messageID), // Content
			waitTime,                              // Randomized WaitTime
		)

		// Serialize the Message object to JSON
		jsonMsg, err := msg.ToJSON()
		if err != nil {
			log.Printf("Failed to serialize message: %v", err)
			continue
		}

		// Start a span for publishing the message
		ctx, span := tracer.Start(context.Background(), "PublishMessage")
		// Inject trace context into the message headers
		headers := make(map[string]string) // Use a map[string]string for the carrier

		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(headers))

		// Convert the map[string]string to amqp.Table
		amqpHeaders := amqp.Table{}
		for k, v := range headers {
			amqpHeaders[k] = v
		}

		err = ch.Publish(
			"",        // exchange
			queueName, // routing key (queue name)
			false,     // mandatory
			false,     // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        []byte(jsonMsg),
				Headers:     amqpHeaders,
			},
		)
		if err != nil {
			span.RecordError(err)
			span.End()
			log.Printf("Failed to publish message: %v", err)
			continue
		}
		span.SetAttributes(
			attribute.String("rabbitmq.queue", queueName),
			attribute.String("rabbitmq.message_id", msg.ID),
			attribute.Int("rabbitmq.wait_time", msg.WaitTime),
		)
		span.End()

		log.Printf("Sent message: %s", jsonMsg)
		messageID++
		time.Sleep(1 * time.Second) // Simulate some delay between messages
	}
}

func initTracer() (*sdktrace.TracerProvider, error) {
	// Create OTLP exporter
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}
