package health

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCheck(t *testing.T) {
	checkFunc := func(ctx context.Context) Status {
		return Status{State: Up}
	}
	check := NewCheck("mycheck", checkFunc)

	assert.Equal(t, "mycheck", check.Name)
	assert.Equal(t, time.Second, check.TTL)
	assert.Equal(t, time.Duration(0), check.Timeout)
}

func TestNew(t *testing.T) {
	checkAFunc := func(ctx context.Context) Status {
		return Status{State: Up}
	}
	checkA := NewCheck("checkA", checkAFunc)

	checkBFunc := func(ctx context.Context) Status {
		return Status{State: Up}
	}
	checkB := NewCheck("checkB", checkBFunc)
	checkB.TTL = time.Second * 2
	checkB.Timeout = time.Millisecond * 400

	checks := []*Check{checkA, checkB}

	healthChecker := Checker{Checks: checks}

	assert.NotNil(t, healthChecker)

	assert.Equal(t, time.Second, checkA.TTL, "Check A TTL was not modified")
	assert.Equal(t, time.Duration(0), checkA.Timeout, "Check A timeout was incorrectly modified")
	assert.Equal(t, time.Second*2, checkB.TTL, "Check B TTL was incorrectly modified")
	assert.Equal(t, time.Millisecond*400, checkB.Timeout, "Check B timeout was incorrectly modified")
}

func TestCheck(t *testing.T) {
	type CustomStatusDetails struct {
		ConnectionCount int
	}

	checkAHealthFunc := func(ctx context.Context) Status {
		return Status{State: Up}
	}
	checkA := NewCheck("checkA", checkAHealthFunc)

	checkBHealthFunc := func(ctx context.Context) Status {
		return Status{
			State:   Degraded,
			Details: CustomStatusDetails{ConnectionCount: 652},
		}
	}
	checkB := NewCheck("checkB", checkBHealthFunc)

	checks := []*Check{checkA, checkB}
	ctx := context.Background()

	healthChecker := Checker{Checks: checks}
	healthChecker.Start(ctx)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	status := healthChecker.Check()

	assert.Equal(t, Degraded, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	assert.Equal(t, checkA.Name, checkAStatus.Name)
	assert.Equal(t, Status{State: Up}, checkAStatus.Status)
	assert.NotEqual(t, 0, checkAStatus.LastExecuted, "Last executed time was not updated")

	checkBStatus := status.CheckStatuses[checkB.Name]
	assert.Equal(t, checkB.Name, checkBStatus.Name)
	assert.Equal(t, Status{State: Degraded, Details: CustomStatusDetails{ConnectionCount: 652}}, checkBStatus.Status)
	assert.NotEqual(t, 0, checkBStatus.LastExecuted, "Last executed time was not updated")
}

func TestCheckInitiallyDown(t *testing.T) {
	checkAHealthFunc := func(ctx context.Context) Status {
		return Status{State: Up}
	}
	checkA := NewCheck("checkA", checkAHealthFunc)

	checkBHealthFunc := func(ctx context.Context) Status {
		return Status{State: Up}
	}
	checkB := NewCheck("checkB", checkBHealthFunc)

	checks := []*Check{checkA, checkB}
	ctx := context.Background()

	healthChecker := Checker{Checks: checks}
	healthChecker.Start(ctx)

	status := healthChecker.Check()

	assert.Equal(t, Down, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	assert.Equal(t, Status{State: Down}, checkAStatus.Status)

	checkBStatus := status.CheckStatuses[checkB.Name]
	assert.Equal(t, Status{State: Down}, checkBStatus.Status)
}

func TestCheckTimeoutEndsExecution(t *testing.T) {
	ttl := time.Duration(time.Millisecond * 100)
	timeout := time.Duration(time.Millisecond * 200)

	type CustomStatusDetails struct {
		ConnectionCount int
	}

	checkAFunc := func(ctx context.Context) Status {
		_, ok := ctx.Deadline()
		assert.True(t, ok, "Check was not supplied with a deadline when timeout is specified")

		select {
		case <-time.After(time.Millisecond * 300):
			// Only return Up after the timeout has been exceeded
			return Status{
				State:   Up,
				Details: CustomStatusDetails{ConnectionCount: 801},
			}
		case <-ctx.Done():
			// Return Degraded if the timeout has been exceeded
			return Status{State: Degraded}
		}
	}
	checkA := NewCheck("checkA", checkAFunc)
	checkA.TTL = ttl
	checkA.Timeout = timeout

	checkBFunc := func(ctx context.Context) Status {
		_, ok := ctx.Deadline()
		assert.False(t, ok, "Check was supplied with a deadline when timeout is not specified")

		return Status{
			State:   Up,
			Details: CustomStatusDetails{ConnectionCount: 347},
		}
	}
	checkB := NewCheck("checkB", checkBFunc)
	checkB.TTL = ttl

	checks := []*Check{checkA, checkB}
	ctx := context.Background()

	healthChecker := Checker{Checks: checks}
	healthChecker.Start(ctx)

	// Wait for goroutines to kick in and timeout to be exceeded
	time.Sleep(time.Millisecond * 400)

	status := healthChecker.Check()

	assert.Equal(t, Degraded, status.State)
	assert.Equal(t, 2, len(status.CheckStatuses))

	checkAStatus := status.CheckStatuses[checkA.Name]
	assert.Equal(t, checkA.Name, checkAStatus.Name)
	assert.Equal(t, Status{State: Degraded}, checkAStatus.Status)
	assert.NotEqual(t, 0, checkAStatus.LastExecuted, "Last executed time was not updated")

	checkBStatus := status.CheckStatuses[checkB.Name]
	assert.Equal(t, checkB.Name, checkBStatus.Name)
	assert.Equal(t, Status{State: Up, Details: CustomStatusDetails{ConnectionCount: 347}}, checkBStatus.Status)
	assert.NotEqual(t, 0, checkBStatus.LastExecuted, "Last executed time was not updated")
}

func TestCheckExecutesOnTimer(t *testing.T) {
	checkACounter := 0
	checkAFunc := func(ctx context.Context) Status {
		checkACounter++
		return Status{State: Up}
	}
	checkA := NewCheck("checkA", checkAFunc)
	checkA.TTL = time.Millisecond * 100

	checkBCounter := 0
	checkBFunc := func(ctx context.Context) Status {
		checkBCounter++
		return Status{State: Down}
	}
	checkB := NewCheck("checkB", checkBFunc)
	checkB.TTL = time.Millisecond * 200

	checks := []*Check{checkA, checkB}
	ctx := context.Background()

	healthChecker := Checker{Checks: checks}
	healthChecker.Start(ctx)

	// Wait for goroutines to kick in and some execution time to pass
	time.Sleep(time.Millisecond * 200)

	healthChecker.Check()

	assert.GreaterOrEqual(t, checkACounter, 2, "Check A did not execute often enough")
	assert.LessOrEqual(t, checkACounter, 3, "Check A executed too many times")

	assert.GreaterOrEqual(t, checkBCounter, 1, "Check B did not execute often enough")
	assert.LessOrEqual(t, checkBCounter, 2, "Check B executed too many times")
}

func TestCheckCancelContextStopsCheck(t *testing.T) {
	checkACounter := 0
	checkAFunc := func(ctx context.Context) Status {
		checkACounter++
		return Status{State: Up}
	}
	checkA := NewCheck("checkA", checkAFunc)
	checkA.TTL = time.Millisecond * 100

	checkBCounter := 0
	checkBFunc := func(ctx context.Context) Status {
		checkBCounter++
		return Status{State: Down}
	}
	checkB := NewCheck("checkB", checkBFunc)
	checkA.TTL = time.Millisecond * 200

	checks := []*Check{checkA, checkB}
	ctx, cancel := context.WithCancel(context.Background())

	healthChecker := Checker{Checks: checks}
	healthChecker.Start(ctx)

	// Wait for goroutines to kick in
	time.Sleep(time.Millisecond * 100)

	assert.GreaterOrEqual(t, checkACounter, 1, "Check A did not execute")
	assert.GreaterOrEqual(t, checkBCounter, 1, "Check B did not execute")

	cancel()

	// Wait for cancel to kick in
	time.Sleep(time.Millisecond * 100)

	checkACounterBefore := checkACounter
	checkBCounterBefore := checkBCounter

	// Wait to see if goroutines are continuing
	time.Sleep(time.Millisecond * 500)

	checkACounterAfter := checkACounter
	checkBCounterAfter := checkBCounter

	assert.Equal(t, checkACounterBefore, checkACounterAfter, "Check A is still executing")
	assert.Equal(t, checkBCounterBefore, checkBCounterAfter, "Check B is still executing")
}
