package health_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jaredpetersen/go-health/health"
	"github.com/stretchr/testify/assert"
)

func TestNewCheck(t *testing.T) {
	checkFunc := func(ctx context.Context) health.Status {
		return health.Status{State: health.StateUp}
	}
	check := health.NewCheck("mycheck", checkFunc)

	assert.Equal(t, "mycheck", check.Name)
	assert.Equal(t, time.Second, check.TTL)
	assert.Equal(t, time.Duration(0), check.Timeout)
}

func TestNew(t *testing.T) {
	healthMonitor := health.New()

	assert.NotNil(t, healthMonitor)
}

func TestCheckEmpty(t *testing.T) {
	healthMonitor := health.New()
	status := healthMonitor.Check()

	expectedStatus := health.MonitorStatus{
		State:         health.StateUp,
		CheckStatuses: make(map[string]health.CheckStatus),
	}

	assert.Equal(t, expectedStatus, status)
}

func TestCheckInitiallyDown(t *testing.T) {
	healthMonitor := health.New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) health.Status {
		return health.Status{State: health.StateUp}
	}
	check := health.NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	status := healthMonitor.Check()

	expectedStatus := health.CheckStatus{
		Status: health.Status{
			State: health.StateDown,
		},
	}

	assert.Equal(t, health.StateDown, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))
	assert.Equal(t, expectedStatus, status.CheckStatuses[check.Name])
}

func TestCheck(t *testing.T) {
	healthMonitor := health.New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) health.Status {
		return health.Status{State: health.StateUp}
	}
	check := health.NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, health.StateUp, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, health.Status{State: health.StateUp}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckDetails(t *testing.T) {
	type CustomStatusDetails struct {
		ConnectionCount int
	}

	healthMonitor := health.New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) health.Status {
		return health.Status{
			State:   health.StateWarn,
			Details: CustomStatusDetails{ConnectionCount: 652},
		}
	}
	check := health.NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, health.StateWarn, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, health.Status{State: health.StateWarn, Details: CustomStatusDetails{ConnectionCount: 652}}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckNoTimeout(t *testing.T) {
	healthMonitor := health.New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) health.Status {
		_, ok := ctx.Deadline()
		assert.False(t, ok, "Check was supplied with a deadline when timeout is not specified")

		return health.Status{State: health.StateWarn}
	}
	check := health.NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, health.StateWarn, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, health.Status{State: health.StateWarn}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckTimeout(t *testing.T) {
	healthMonitor := health.New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) health.Status {
		_, ok := ctx.Deadline()
		assert.True(t, ok, "Check was not supplied with a deadline when timeout is specified")

		return health.Status{State: health.StateWarn}
	}
	check := health.NewCheck("check", checkHealthFunc)
	check.Timeout = time.Second * 1
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, health.StateWarn, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, health.Status{State: health.StateWarn}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckMultiple(t *testing.T) {
	type CustomStatusDetails struct {
		ConnectionCount int
	}

	healthMonitor := health.New()
	ctx := context.Background()

	checkAHealthFunc := func(ctx context.Context) health.Status {
		return health.Status{State: health.StateUp}
	}
	checkA := health.NewCheck("checkA", checkAHealthFunc)
	healthMonitor.Monitor(ctx, checkA)

	checkBHealthFunc := func(ctx context.Context) health.Status {
		return health.Status{
			State:   health.StateWarn,
			Details: CustomStatusDetails{ConnectionCount: 104},
		}
	}
	checkB := health.NewCheck("checkB", checkBHealthFunc)
	healthMonitor.Monitor(ctx, checkB)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, health.StateWarn, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	expectedCheckAStatus := health.Status{State: health.StateUp}
	assert.Equal(t, expectedCheckAStatus, checkAStatus.Status)
	assert.NotEqual(t, 0, checkAStatus.Timestamp, "Check status timestamp was not updated")

	checkBStatus := status.CheckStatuses[checkB.Name]
	expectedCheckBStatus := health.Status{
		State:   health.StateWarn,
		Details: CustomStatusDetails{ConnectionCount: 104},
	}
	assert.Equal(t, expectedCheckBStatus, checkBStatus.Status)
	assert.NotEqual(t, 0, checkBStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckMultipleVariadicMonitor(t *testing.T) {
	type CustomStatusDetails struct {
		ConnectionCount int
	}

	healthMonitor := health.New()
	ctx := context.Background()

	checkAHealthFunc := func(ctx context.Context) health.Status {
		return health.Status{State: health.StateUp}
	}
	checkA := health.NewCheck("checkA", checkAHealthFunc)

	checkBHealthFunc := func(ctx context.Context) health.Status {
		return health.Status{
			State:   health.StateWarn,
			Details: CustomStatusDetails{ConnectionCount: 104},
		}
	}
	checkB := health.NewCheck("checkB", checkBHealthFunc)

	// Use variadic argument for monitor
	healthMonitor.Monitor(ctx, checkA, checkB)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, health.StateWarn, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	expectedCheckAStatus := health.Status{State: health.StateUp}
	assert.Equal(t, expectedCheckAStatus, checkAStatus.Status)
	assert.NotEqual(t, 0, checkAStatus.Timestamp, "Check status timestamp was not updated")

	checkBStatus := status.CheckStatuses[checkB.Name]
	expectedCheckBStatus := health.Status{
		State:   health.StateWarn,
		Details: CustomStatusDetails{ConnectionCount: 104},
	}
	assert.Equal(t, expectedCheckBStatus, checkBStatus.Status)
	assert.NotEqual(t, 0, checkBStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckTimeoutEndsExecution(t *testing.T) {
	healthMonitor := health.New()
	ctx := context.Background()

	ttl := time.Duration(time.Millisecond * 100)

	checkFunc := func(ctx context.Context) health.Status {
		select {
		case <-time.After(time.Millisecond * 300):
			// Only return Up after the timeout has been exceeded
			return health.Status{State: health.StateUp}
		case <-ctx.Done():
			// Return Degraded if the timeout has been exceeded
			return health.Status{State: health.StateWarn}
		}
	}

	checkA := health.NewCheck("checkA", checkFunc)
	checkA.TTL = ttl
	checkA.Timeout = time.Duration(time.Millisecond * 200)
	healthMonitor.Monitor(ctx, checkA)

	checkB := health.NewCheck("checkB", checkFunc)
	checkB.TTL = ttl
	checkA.Timeout = time.Duration(time.Second)
	healthMonitor.Monitor(ctx, checkB)

	// Wait for goroutines to kick in and checkA timeout to be exceeded
	time.Sleep(time.Millisecond * 400)

	status := healthMonitor.Check()

	assert.Equal(t, health.StateWarn, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	assert.Equal(t, health.Status{State: health.StateWarn}, checkAStatus.Status)
	assert.NotEqual(t, 0, checkAStatus.Timestamp, "Last executed time was not updated")

	checkBStatus := status.CheckStatuses[checkB.Name]
	assert.Equal(t, health.Status{State: health.StateUp}, checkBStatus.Status)
	assert.NotEqual(t, 0, checkBStatus.Timestamp, "Last executed time was not updated")
}

func TestCheckExecutesOnTimer(t *testing.T) {
	healthMonitor := health.New()
	ctx := context.Background()

	var atomicCheckACounter int32
	checkAFunc := func(ctx context.Context) health.Status {
		atomic.AddInt32(&atomicCheckACounter, 1)
		return health.Status{State: health.StateUp}
	}
	checkA := health.NewCheck("checkA", checkAFunc)
	checkA.TTL = time.Millisecond * 100
	healthMonitor.Monitor(ctx, checkA)

	var atomicCheckBCounter int32
	checkBFunc := func(ctx context.Context) health.Status {
		atomic.AddInt32(&atomicCheckBCounter, 1)
		return health.Status{State: health.StateDown}
	}
	checkB := health.NewCheck("checkB", checkBFunc)
	checkB.TTL = time.Millisecond * 200
	healthMonitor.Monitor(ctx, checkB)

	// Wait for goroutines to kick in and some execution time to pass
	time.Sleep(time.Millisecond * 200)

	healthMonitor.Check()

	checkACounter := atomic.LoadInt32(&atomicCheckACounter)
	assert.GreaterOrEqual(t, checkACounter, int32(2), "Check A did not execute often enough")
	assert.LessOrEqual(t, checkACounter, int32(3), "Check A executed too many times")

	checkBCounter := atomic.LoadInt32(&atomicCheckBCounter)
	assert.GreaterOrEqual(t, checkBCounter, int32(1), "Check B did not execute often enough")
	assert.LessOrEqual(t, checkBCounter, int32(2), "Check B executed too many times")
}

func TestCheckCancelContextStopsCheck(t *testing.T) {
	healthMonitor := health.New()
	ctx, cancel := context.WithCancel(context.Background())

	var atomicCheckACounter int32
	checkAFunc := func(ctx context.Context) health.Status {
		atomic.AddInt32(&atomicCheckACounter, 1)
		return health.Status{State: health.StateUp}
	}
	checkA := health.NewCheck("checkA", checkAFunc)
	checkA.TTL = time.Millisecond * 100
	healthMonitor.Monitor(ctx, checkA)

	var atomicCheckBCounter int32
	checkBFunc := func(ctx context.Context) health.Status {
		atomic.AddInt32(&atomicCheckBCounter, 1)
		return health.Status{State: health.StateDown}
	}
	checkB := health.NewCheck("checkB", checkBFunc)
	checkA.TTL = time.Millisecond * 200
	healthMonitor.Monitor(ctx, checkB)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	assert.GreaterOrEqual(t, atomic.LoadInt32(&atomicCheckACounter), int32(1), "Check A did not execute")
	assert.GreaterOrEqual(t, atomic.LoadInt32(&atomicCheckBCounter), int32(1), "Check B did not execute")

	// Stop all execution
	cancel()

	// Wait for cancel to kick in
	time.Sleep(time.Millisecond * 100)

	checkACounterBefore := atomic.LoadInt32(&atomicCheckACounter)
	checkBCounterBefore := atomic.LoadInt32(&atomicCheckBCounter)

	// Wait to see if goroutines are continuing
	time.Sleep(time.Millisecond * 500)

	checkACounterAfter := atomic.LoadInt32(&atomicCheckACounter)
	checkBCounterAfter := atomic.LoadInt32(&atomicCheckBCounter)

	assert.Equal(t, checkACounterBefore, checkACounterAfter, "Check A is still executing")
	assert.Equal(t, checkBCounterBefore, checkBCounterAfter, "Check B is still executing")
}

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

func Example_hhtp() {
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
