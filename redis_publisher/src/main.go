/*
This is based off the redis demo I've made here:
https://github.com/felixfyx/go-redis-demo

Based on the server
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type LogText struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

/*
Note as of 24/8/2023:

Sometimes information about a runtime environment can change dynamically or be
delayed from startup. Instead of continuously recreating and distributing a
TracerProvider with an immutable Resource or delaying the startup of your
application on a slow-loading piece of information, annotate the created spans
dynamically using a SpanProcessor.
*/

var (
	// owner represents the owner of the application. In this example it is
	// stored as a simple string, but in real-world use this may be the
	// response to an asynchronous request.

	// Modified from original code, it used to be 'unknown' and then
	// set to something else later to show who/what was using this
	owner    = "redis_publisher"
	ownerKey = attribute.Key("owner")
)

type Annotator struct {
	// Annotator is a SpanProcessor that adds attributes to all started spans.
	// AttrsFunc is called when a span is started. The attributes it returns
	// are set on the Span being started.
	AttrsFunc func() []attribute.KeyValue
}

func (a Annotator) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	s.SetAttributes(a.AttrsFunc()...)
}
func (a Annotator) Shutdown(context.Context) error   { return nil }
func (a Annotator) ForceFlush(context.Context) error { return nil }
func (a Annotator) OnEnd(s sdktrace.ReadOnlySpan) {
	attr := s.Attributes()[0]
	fmt.Printf("%s: %s\n", attr.Key, attr.Value.AsString())
}

// Logger to be used with zipkin
var logger = log.New(os.Stderr, "zipkin-example",
	log.Ldate|log.Ltime|log.Llongfile)

// initTracer creates a new trace provider instance and registers it as global trace provider.
func initTracer(url string) (func(context.Context) error, error) {
	// Create Zipkin Exporter and install it as a global tracer.
	//
	// For demoing purposes, always sample. In a production application, you should
	// configure the sampler to a trace.ParentBased(trace.TraceIDRatioBased) set at the desired
	// ratio.
	exporter, err := zipkin.New(
		url,
		zipkin.WithLogger(logger),
	)
	if err != nil {
		return nil, err
	}

	batcher := sdktrace.NewBatchSpanProcessor(exporter)

	// annotations
	a := Annotator{
		AttrsFunc: func() []attribute.KeyValue {
			return []attribute.KeyValue{ownerKey.String(owner)}
		},
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(batcher),
		sdktrace.WithSpanProcessor(a),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("publisher-metrics"),
		)),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func main() {
	// Setup redis related stuff
	// Read config off env to get redis host and port number
	var host = os.Getenv("redis_host")
	var port = os.Getenv("redis_port")

	fmt.Println("--- Setting up Redis Publisher stuff ---")
	var redisClient = redis.NewClient(&redis.Options{
		Addr: host + ":" + port,
	})

	fmt.Println("--- Setting up Zipkin stuff ---")
	// Setup zipkin related stuff
	var zHost = os.Getenv("zipkin_host")
	var zPort = os.Getenv("zipkin_port")
	url := "http://" + zHost + ":" + zPort + "/api/v2/spans"

	app := fiber.New()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Initializing tracer
	shutdown, err := initTracer(url)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()
	fmt.Println("Initialize tracer to: " + url)

	// Creating a unique tracer
	tracer := otel.GetTracerProvider().Tracer("publisher-tracer")
	ctx, span := tracer.Start(ctx, "redisPublisher", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End() // Attempt to end the span during cleanup

	fmt.Println("Running Publisher...")
	// Endpoint stuff
	app.Post("/", func(c *fiber.Ctx) error {
		// Creating a new tracer for calculating how long it takes for this
		// function to clear
		tr := otel.GetTracerProvider().Tracer("publisher-post")
		_, span := tr.Start(ctx, "post-log-event")

		// This will end the span that is post-log-event from the publisher-post
		// tracer
		defer span.End()

		logtext := new(LogText)

		if err := c.BodyParser(logtext); err != nil {
			panic(err)
		}

		payload, err := json.Marshal(logtext)
		if err != nil {
			panic(err)
		}

		err = redisClient.Publish(ctx, "send-log-data", payload).Err()
		if err != nil {
			panic(err)
		}

		c.WriteString("Sent payload: " + string(payload))

		return c.SendStatus(http.StatusOK)
	})

	app.Listen(":8080")
}
