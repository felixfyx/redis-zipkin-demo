/*
This is based off the redis demo I've made here: https://github.com/felixfyx/go-redis-demo

Based on the client
*/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type LogText struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

var (
	// owner represents the owner of the application. In this example it is
	// stored as a simple string, but in real-world use this may be the
	// response to an asynchronous request.

	// Modified from original code, it used to be 'unknown' and then
	// set to something else later to show who/what was using this
	owner    = "redis_subscriber"
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
			semconv.ServiceName("subscriber-metrics"),
		)),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func main() {
	fmt.Println("Reading config...")
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file, %s", err)
	}

	// Create redisClient
	var host = viper.GetString("server.host")
	var port = viper.GetString("server.port")

	fmt.Println("--- Setting up Redis Subscriber stuff ---")
	var redisClient = redis.NewClient(&redis.Options{
		Addr: host + ":" + port,
	})

	fmt.Println("--- Setting up Zipkin stuff ---")
	// Setup zipkin related stuff
	var zHost = viper.GetString("zipkin.host")
	var zPort = viper.GetString("zipkin.port")
	url := "http://" + zHost + ":" + zPort + "/api/v2/spans"

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
	tracer := otel.GetTracerProvider().Tracer("subscriber-tracer")
	ctx, span := tracer.Start(ctx, "redisSubscriber", trace.WithSpanKind(trace.SpanKindServer))
	defer span.End() // Attempt to end the span during cleanup

	fmt.Println("Running client...")

	subscriber := redisClient.Subscribe(ctx, "send-log-data")
	logtext := LogText{}

	tr := otel.GetTracerProvider().Tracer("subscriber-receive")

	for {
		startTime := time.Now()
		// Creating a new tracer for calculating how long it takes to receive
		// one message between another
		_, span := tr.Start(ctx, "received-log-event", trace.WithTimestamp(startTime))

		// This should be used in a channel as opposed to
		// running on a main thread
		msg, err := subscriber.ReceiveMessage(ctx)
		if err != nil {
			panic(err)
		}

		if err := json.Unmarshal([]byte(msg.Payload), &logtext); err != nil {
			panic(err)
		}

		fmt.Println("Received message from " + msg.Channel + " channel.")
		fmt.Printf("%+v\n", logtext)

		// End the span
		// HACK: MANUALLY SETTING SPAN TIME
		span.End(trace.WithTimestamp(startTime.Add(time.Hour)))
	}
}
