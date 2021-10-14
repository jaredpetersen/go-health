// Package health is a barebones, detailed health check library.
package health

import (
	"context"
	"sync"
	"time"
)

// State represents the health of the resource being checked as a simple value.
type State int

// States must be ordered from most degraded to least degraded here so that checks work their way up to the best state.

const (
	// StateDown indicates unhealthy resource state.
	StateDown State = iota
	// StateWarn indicates healthy resource state with some concerns.
	StateWarn
	// StateUp indicates a completely healthy resource state withot any concerns.
	StateUp
)

// MonitorStatus represents the health of the all of the resources being checked.
type MonitorStatus struct {
	// State is a high level indicator for the health of all of the checks. It combines all of the check states
	// together and bubbles up the most degraded. For example, if there are three checks and one has a state of
	// StateDown, the overall state will be StateDown.
	State State
	// CheckStatuses contains all of the resource statuses. The map key is the name of the check.
	CheckStatuses map[string]CheckStatus
}

// CheckStatus indicates the health of an individual check and when that information was retrieved.
type CheckStatus struct {
	// Status of the resource.
	Status Status
	// Timestamp is the time the status was determined.
	Timestamp time.Time
}

// Status indicates resource health state and contains any additional, arbitrary details that may be relevant.
type Status struct {
	// State is a high level indicator for resource health.
	State State
	// Details is for any additional information about the resource that you want to expose.
	Details interface{}
}

// Func is a function used to determine resource health.
type CheckFunc func(ctx context.Context) Status

// Check represents a resource to be checked. The check function is used to determine resource health and is executed
// on a cadence defined by the configured TTL.
type Check struct {
	// Name of the check. Must be unique.
	Name string
	// CheckFunc is a function used to determine resource health. This will be executed on a cadence as defined by the
	// configured TTL. It is your responsibility to ensure that this function respects the provided context so that the
	// logic may be terminated early. The provided context will be given a deadline if the check is configured with a
	// timeout.
	Func CheckFunc
	// TTL is the time that should be waited on between executions of the health check function.
	TTL time.Duration
	// Timeout is the max time that the check function may execute in before the provided context communicates
	// termination.
	Timeout time.Duration
}

// NewCheck creates a new health check with suitable default values.
//
// TTL is set to a duration of 1 second (1 second cache between executions of the check function).
//
// Timeout is left at its zero-value, meaning that there is no deadline for completion. It is recommended that you
// configure a timeout yourself but this is not required.
func NewCheck(name string, checkFunc CheckFunc) Check {
	return Check{
		Name: name,
		Func: checkFunc,
		TTL:  time.Second * 1,
	}
}

// Monitor coordinates checks and executes their status functions to determine application health.
type Monitor struct {
	// checkStatuses is a cache of all of the check function results, the key being the name of the check.
	checkStatuses map[string]CheckStatus
	// mtx is a read-write mutex used to coordinate reads and writes to the checkStatuses cache.
	mtx sync.RWMutex
}

// New creates a health monitor that monitors the provided checks. The return value will never be nil.
func New() *Monitor {
	// Cache the check status results in a map organized by check name as the key.
	checkStatuses := make(map[string](CheckStatus))

	return &Monitor{checkStatuses: checkStatuses}
}

// setCheckStatus updates the check status cache in a thread-safe manner using the monitor mutex.
func (mtr *Monitor) setCheckStatus(checkName string, checkStatus CheckStatus) {
	mtr.mtx.Lock()
	mtr.checkStatuses[checkName] = checkStatus
	mtr.mtx.Unlock()
}

// Monitor starts a goroutine the executes the checks' check function and caches the result. This goroutine will wait
// between polls as defined by check's TTL to avoid spamming the resource being evaluated. If a timeout is set on the
// check, the context provided to Monitor will be wrapped in a deadline context and provided to the check function to
// facilitate early termination.
func (mtr *Monitor) Monitor(ctx context.Context, check Check) {
	// Initialize the cache as StateDown
	initialStatus := CheckStatus{
		Status: Status{
			State: StateDown,
		},
	}
	mtr.setCheckStatus(check.Name, initialStatus)

	// Start polling the check resource asynchronously
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var checkStatus CheckStatus
				if check.Timeout > 0 {
					checkStatus = executeCheckWithTimeout(ctx, check)
				} else {
					checkStatus = executeCheck(ctx, check)
				}

				mtr.setCheckStatus(check.Name, checkStatus)
				time.Sleep(check.TTL)
			}
		}
	}()
}

// Check returns the latest cached status for all of the configured checks.
func (mtr *Monitor) Check() MonitorStatus {
	// Use StateUp as the initial state so that it may be overidden by the checks if necessary.
	// If checks are not configured, then we also default to StateUp.
	state := StateUp

	// Create a copy of the internal check status map so that we can return it without it being impacted by updates
	// being performed by the monitor goroutines.
	checkStatuses := make(map[string]CheckStatus)

	mtr.mtx.RLock()

	for checkName, checkStatus := range mtr.checkStatuses {
		state = compareState(state, checkStatus.Status.State)
		checkStatuses[checkName] = checkStatus
	}

	monitorStatus := MonitorStatus{State: state, CheckStatuses: checkStatuses}

	mtr.mtx.RUnlock()

	return monitorStatus
}

// executeCheck executes the check function using the provided context and updates the check information.
func executeCheck(ctx context.Context, check Check) CheckStatus {
	return CheckStatus{
		Status:    check.Func(ctx),
		Timestamp: time.Now(),
	}
}

// executeCheck executes the check function using the provided context, wrapped with a deadline set to the check's
// configured timeout, and updates the check information.
func executeCheckWithTimeout(ctx context.Context, check Check) CheckStatus {
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, check.Timeout)
	defer cancelTimeout()

	return executeCheck(timeoutCtx, check)
}

// compareState compares states and returns the most degraded state of the two.
func compareState(stateA State, stateB State) State {
	var state State

	if stateA == StateDown || stateB == StateDown {
		state = StateDown
	} else if stateA == StateWarn || stateB == StateWarn {
		state = StateWarn
	} else {
		state = StateUp
	}

	return state
}
