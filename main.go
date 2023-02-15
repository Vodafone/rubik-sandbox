// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command jaeger is an example program that creates spans
// and uploads to Jaeger.
package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

const (
	service     = "trace-demo"
	environment = "production"
	id          = 1
)

// tracerProvider returns an OpenTelemetry TracerProvider configured to use
// the Jaeger exporter that will send spans to the provided url. The returned
// TracerProvider will also use a Resource configured with all the information
// about the application.
func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(service),
			attribute.String("environment", environment),
			attribute.Int64("ID", id),
		)),
	)
	return tp, nil
}

func main() {

	jaegerCollector := flag.String("j", "http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces", "jaeger collector like http://localhost:14268/api/traces or http://jaeger-collector.jaeger.svc.cluster.local:14268/api/traces")
	iterations := flag.Int("i", 1, "iterations of busywork")
	numberOfGoroutines := flag.Int("g", 1, "number of goRoutines")
	maxGo := flag.Int("maxgo", 0, "number of set GOMAXPROCS")

	flag.Parse()

	runtime.GOMAXPROCS(*maxGo)

	tp, err := tracerProvider(*jaegerCollector)
	if err != nil {
		log.Fatal(err)
	}

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cleanly shutdown and flush telemetry when the application exits.
	defer func(ctx context.Context) {
		// Do not make the application hang when it is shutdown.
		ctx, cancel = context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}(ctx)

	hostname, _ := os.Hostname()

	tr := tp.Tracer("component-main-" + hostname)

	ctx, span := tr.Start(ctx, "foo")
	defer span.End()

	// do some work
	bar(ctx, *iterations)

	// do some 'concurrent' work
	channelCap := *numberOfGoroutines
	channel := make(chan int, channelCap)
	for i := 0; i < channelCap; i++ {
		go barChan(ctx, *iterations, channel)
	}

	for i := 0; i < channelCap; i++ {
		<-channel
	}

}

func barChan(ctx context.Context, iterations int, ch chan int) {
	bar(ctx, int(iterations))
	ch <- 1
}

func bar(ctx context.Context, iterations int) {
	// Use the global TracerProvider.
	tr := otel.Tracer("component-bar")
	_, span := tr.Start(ctx, "bar")
	rt_ngor := strconv.Itoa(runtime.NumGoroutine())
	rt_ngmp := strconv.Itoa(runtime.GOMAXPROCS(0))
	rt_ncpu := strconv.Itoa(runtime.NumCPU())
	rt_work := strconv.Itoa(iterations)
	span.SetAttributes(attribute.Key("NumGoroutine").String(rt_ngor))
	span.SetAttributes(attribute.Key("GOMAXPROCS").String(rt_ngmp))
	span.SetAttributes(attribute.Key("NumCPU").String(rt_ncpu))
	span.SetAttributes(attribute.Key("Work").String(rt_work))

	defer span.End()

	for i := 0; i < iterations; i++ {
		randNum1 := rand.Intn(1000)
		randNum2 := rand.Intn(10000)
		randRes := randNum1 * randNum2
		if randRes < 0 {
			randRes = 0
		}
	}
}
