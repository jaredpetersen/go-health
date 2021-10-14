package health_test

import (
	"context"
	"net/http"
	"time"

	"github.com/jaredpetersen/go-health/health"
)

func Example() {
	// Create the health monitor that will be polling the resources.
	healthMonitor := health.New()

	// Prepare the context -- this can be used to stop async monitoring.
	ctx := context.Background()

	// Create your health checks.
	fooHealthCheckFunc := func(ctx context.Context) health.Status {
		return health.Status{State: health.StateDown}
	}
	fooHealthCheck := health.NewCheck("foo", fooHealthCheckFunc)
	fooHealthCheck.Timeout = time.Second * 2
	healthMonitor.Monitor(ctx, fooHealthCheck)

	barHealthCheckFunc := func(ctx context.Context) health.Status {
		return health.Status{State: health.StateUp}
	}
	barHealthCheck := health.NewCheck("bar", barHealthCheckFunc)
	barHealthCheck.Timeout = time.Second * 2
	healthMonitor.Monitor(ctx, barHealthCheck)

	// Wait for goroutines to kick off
	time.Sleep(time.Millisecond * 100)

	// Retrieve the most recent cached result for all of the checks.
	healthMonitor.Check()
}

func Example_http() {
	// Create the health monitor that will be polling the resources.
	healthMonitor := health.New()

	// Prepare the context -- this can be used to stop async monitoring.
	ctx := context.Background()

	// Set up a generic health checker, though anything that implements the check function will do.
	httpClient := http.Client{}
	type HTTPHealthCheckDetails struct {
		ResponseTime time.Duration
	}
	httpHealthCheckFunc := func(url string) health.CheckFunc {
		statusDown := health.Status{State: health.StateDown}

		return func(ctx context.Context) health.Status {
			// Create a HTTP request that terminates when the context is terminated.
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return statusDown
			}

			// Execute the HTTP request
			requestStart := time.Now()
			res, err := httpClient.Do(req)
			responseTime := time.Since(requestStart)
			if err != nil {
				return statusDown
			}

			if res.StatusCode == http.StatusOK {
				return health.Status{
					State:   health.StateUp,
					Details: HTTPHealthCheckDetails{ResponseTime: responseTime},
				}
			} else {
				return statusDown
			}
		}
	}

	// Create your health checks.
	exampleHealthCheckFunc := httpHealthCheckFunc("http://example.com")
	exampleHealthCheck := health.NewCheck("example", exampleHealthCheckFunc)
	exampleHealthCheck.Timeout = time.Second * 2
	healthMonitor.Monitor(ctx, exampleHealthCheck)

	godevHealthCheckFunc := httpHealthCheckFunc("https://go.dev")
	godevHealthCheck := health.NewCheck("godev", godevHealthCheckFunc)
	godevHealthCheck.Timeout = time.Second * 2
	healthMonitor.Monitor(ctx, godevHealthCheck)

	// Wait for goroutines to kick off
	time.Sleep(time.Second * 2)

	// Retrieve the most recent cached result for all of the checks.
	healthMonitor.Check()
}
