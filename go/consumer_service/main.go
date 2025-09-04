package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	"go.opentelemetry.io/otel/trace"
)

const (
	rabbitMQURL = "amqp://admin:adminpassword@my-rabbitmq.demo.svc.cluster.local:5672/"
	queueName   = "work"
	serviceName = "consumer-service"
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

	// Declare the queue
	_, err = ch.QueueDeclare(
		queueName, // name of the queue
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare the queue: %v", err)
	}

	// Consume messages from the queue
	msgs, err := ch.Consume(
		queueName, // queue name
		"",        // consumer tag
		true,      // auto-acknowledge
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	log.Printf("Listening for messages on queue: %s", queueName)

	// Process messages
	for d := range msgs {

		// Extract trace context from the message headers
		headers := make(map[string]string)
		for k, v := range d.Headers {
			if strVal, ok := v.(string); ok { // Only include string values
				headers[k] = strVal
			}
		}
		carrier := propagation.MapCarrier(headers)

		ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
		fmt.Println("Extracted context: ", ctx)
		// Start a new trace span for processing the message
		spanContext := trace.SpanContextFromContext(ctx)
		if spanContext.IsValid() {
			log.Printf("Parent Trace ID: %s", spanContext.TraceID().String())
		} else {
			log.Println("No parent trace ID found")
		}

		_, span := tracer.Start(ctx, "ProcessMessage")

		span.SetAttributes(attribute.String("rabbitmq.queue", queueName))
		fmt.Println("parent trace ID: ", span.Parent().TraceID().String())

		// Unmarshal the message
		var msg message.Message
		err := json.Unmarshal(d.Body, &msg)
		if err != nil {
			span.RecordError(err)
			span.End()
			log.Printf("Failed to unmarshal message: %v", err)
			continue
		}

		span.SetAttributes(
			attribute.String("rabbitmq.message_id", msg.ID),
			attribute.String("rabbitmq.message_content", msg.Content),
			attribute.Int("rabbitmq.wait_time", msg.WaitTime),
		)

		// Simulate processing by waiting for the specified WaitTime
		log.Printf("Processing message: %s (WaitTime: %d seconds)", msg.Content, msg.WaitTime)
		time.Sleep(time.Duration(msg.WaitTime) * time.Second)

		// End the span
		span.End()
	}

	log.Println("Consumer shutting down...")
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
