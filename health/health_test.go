package health

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCheck(t *testing.T) {
	checkFunc := func(ctx context.Context) Status {
		return Status{State: StateUp}
	}
	check := NewCheck("mycheck", checkFunc)

	assert.Equal(t, "mycheck", check.Name)
	assert.Equal(t, time.Second, check.TTL)
	assert.Equal(t, time.Duration(0), check.Timeout)
}

func TestNew(t *testing.T) {
	healthMonitor := New()

	assert.NotNil(t, healthMonitor)
}

func TestCheckEmpty(t *testing.T) {
	healthMonitor := New()
	healthStatus := healthMonitor.Check()

	assert.Equal(t, MonitorStatus{State: StateUp, CheckStatuses: make(map[string]CheckStatus)}, healthStatus)
}

func TestCheckInitiallyDown(t *testing.T) {
	healthMonitor := New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) Status {
		return Status{State: StateUp}
	}
	check := NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	status := healthMonitor.Check()

	assert.Equal(t, StateDown, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))
	assert.Equal(t, CheckStatus{Status: Status{State: StateDown}}, status.CheckStatuses[check.Name])
}

func TestCheck(t *testing.T) {
	healthMonitor := New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) Status {
		return Status{State: StateUp}
	}
	check := NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, StateUp, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, Status{State: StateUp}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckDetails(t *testing.T) {
	type CustomStatusDetails struct {
		ConnectionCount int
	}

	healthMonitor := New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) Status {
		return Status{
			State:   StateWarn,
			Details: CustomStatusDetails{ConnectionCount: 652},
		}
	}
	check := NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, StateWarn, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, Status{State: StateWarn, Details: CustomStatusDetails{ConnectionCount: 652}}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckNoTimeout(t *testing.T) {
	healthMonitor := New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) Status {
		_, ok := ctx.Deadline()
		assert.False(t, ok, "Check was supplied with a deadline when timeout is not specified")

		return Status{State: StateWarn}
	}
	check := NewCheck("check", checkHealthFunc)
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, StateWarn, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, Status{State: StateWarn}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckTimeout(t *testing.T) {
	healthMonitor := New()
	ctx := context.Background()

	checkHealthFunc := func(ctx context.Context) Status {
		_, ok := ctx.Deadline()
		assert.True(t, ok, "Check was not supplied with a deadline when timeout is specified")

		return Status{State: StateWarn}
	}
	check := NewCheck("check", checkHealthFunc)
	check.Timeout = time.Second * 1
	healthMonitor.Monitor(ctx, check)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, StateWarn, status.State)
	assert.Equal(t, 1, len(status.CheckStatuses))

	checkStatus := status.CheckStatuses[check.Name]
	assert.Equal(t, Status{State: StateWarn}, checkStatus.Status)
	assert.NotEqual(t, 0, checkStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckMultiple(t *testing.T) {
	type CustomStatusDetails struct {
		ConnectionCount int
	}

	healthMonitor := New()
	ctx := context.Background()

	checkAHealthFunc := func(ctx context.Context) Status {
		return Status{State: StateUp}
	}
	checkA := NewCheck("checkA", checkAHealthFunc)
	healthMonitor.Monitor(ctx, checkA)

	checkBHealthFunc := func(ctx context.Context) Status {
		return Status{
			State:   StateWarn,
			Details: CustomStatusDetails{ConnectionCount: 104},
		}
	}
	checkB := NewCheck("checkB", checkBHealthFunc)
	healthMonitor.Monitor(ctx, checkB)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthMonitor.Check()

	assert.Equal(t, StateWarn, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	assert.Equal(t, Status{State: StateUp}, checkAStatus.Status)
	assert.NotEqual(t, 0, checkAStatus.Timestamp, "Check status timestamp was not updated")

	checkBStatus := status.CheckStatuses[checkB.Name]
	assert.Equal(t, Status{State: StateWarn, Details: CustomStatusDetails{ConnectionCount: 104}}, checkBStatus.Status)
	assert.NotEqual(t, 0, checkBStatus.Timestamp, "Check status timestamp was not updated")
}

func TestCheckTimeoutEndsExecution(t *testing.T) {
	healthMonitor := New()
	ctx := context.Background()

	ttl := time.Duration(time.Millisecond * 100)

	checkFunc := func(ctx context.Context) Status {
		select {
		case <-time.After(time.Millisecond * 300):
			// Only return Up after the timeout has been exceeded
			return Status{State: StateUp}
		case <-ctx.Done():
			// Return Degraded if the timeout has been exceeded
			return Status{State: StateWarn}
		}
	}

	checkA := NewCheck("checkA", checkFunc)
	checkA.TTL = ttl
	checkA.Timeout = time.Duration(time.Millisecond * 200)
	healthMonitor.Monitor(ctx, checkA)

	checkB := NewCheck("checkB", checkFunc)
	checkB.TTL = ttl
	checkA.Timeout = time.Duration(time.Second)
	healthMonitor.Monitor(ctx, checkB)

	// Wait for goroutines to kick in and checkA timeout to be exceeded
	time.Sleep(time.Millisecond * 400)

	status := healthMonitor.Check()

	assert.Equal(t, StateWarn, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	assert.Equal(t, Status{State: StateWarn}, checkAStatus.Status)
	assert.NotEqual(t, 0, checkAStatus.Timestamp, "Last executed time was not updated")

	checkBStatus := status.CheckStatuses[checkB.Name]
	assert.Equal(t, Status{State: StateUp}, checkBStatus.Status)
	assert.NotEqual(t, 0, checkBStatus.Timestamp, "Last executed time was not updated")
}

func TestCheckExecutesOnTimer(t *testing.T) {
	healthMonitor := New()
	ctx := context.Background()

	var atomicCheckACounter int32
	checkAFunc := func(ctx context.Context) Status {
		atomic.AddInt32(&atomicCheckACounter, 1)
		return Status{State: StateUp}
	}
	checkA := NewCheck("checkA", checkAFunc)
	checkA.TTL = time.Millisecond * 100
	healthMonitor.Monitor(ctx, checkA)

	var atomicCheckBCounter int32
	checkBFunc := func(ctx context.Context) Status {
		atomic.AddInt32(&atomicCheckBCounter, 1)
		return Status{State: StateDown}
	}
	checkB := NewCheck("checkB", checkBFunc)
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
	healthMonitor := New()
	ctx, cancel := context.WithCancel(context.Background())

	var atomicCheckACounter int32
	checkAFunc := func(ctx context.Context) Status {
		atomic.AddInt32(&atomicCheckACounter, 1)
		return Status{State: StateUp}
	}
	checkA := NewCheck("checkA", checkAFunc)
	checkA.TTL = time.Millisecond * 100
	healthMonitor.Monitor(ctx, checkA)

	var atomicCheckBCounter int32
	checkBFunc := func(ctx context.Context) Status {
		atomic.AddInt32(&atomicCheckBCounter, 1)
		return Status{State: StateDown}
	}
	checkB := NewCheck("checkB", checkBFunc)
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
