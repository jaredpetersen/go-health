// Package health is a barebones, detailed health check library.
package health

import (
	"context"
	"time"
)

// State represents the health of the resource being checked as a simple value.
type State int

// States must be ordered from most degraded to least degraded here so that checks work their way up to the best state.

const (
	// StateDown represents unhealthy resource state.
	StateDown State = iota
	// StateWarn represents healthy resource state with some concerns.
	StateWarn
	// StateUp represents a completely healthy resource state withot any concerns.
	StateUp
)

// CheckerStatus represents the health of the all of the resources being checked.
type CheckerStatus struct {
	// State is a high level indicator for the health of all of the checks. It combines all of the check states
	// together and bubbles up the most degraded. For example, if there are three checks and one has a state of
	// health.StateDown, the overall state will be health.StateDown.
	State State
	// CheckStatuses contains all of the resource statuses. The key of the map is the check name.
	CheckStatuses map[string]CheckStatus
}

// CheckStatus represents the health of the individual check.
type CheckStatus struct {
	// Name of the check that produced the status.
	Name string
	// Status of the resource.
	Status Status
	// LastExecuted is the last time the check executed
	LastExecuted time.Time
}

// Status is a wrapper for resource health state and any additional, arbitrary details that you may wish to include.
type Status struct {
	// State is a high level indicator for resource health
	State State
	// Details is for any additional information about the resource that you want to expose
	Details interface{}
}

// Check represents a resource to be checked. The check function is used to determine resource health and is executed
// on a cadence defined by the configured TTL. The result of the function execution is then cached on the check for
// individual resource health status consumption if desired.
type Check struct {
	// Name of the check.
	Name string
	// Func is the resource health check function to be executed when determining health. This will be executed on a
	// cadence as defined by the configured TTL. It is your responsibility to ensure that this function respects the
	// provided context so that the logic may be terminated early. The provided context will be given a deadline if the
	// check is configured with a timeout.
	Func func(ctx context.Context) Status
	// TTL is the time between executions of the health check function. After the function completes, the checker will
	// respect this value by waiting for this time to elapse before executing the function again and caching the
	// results.
	TTL time.Duration
	// Timeout is the max time that the check function may execute in before the provided context communicates
	// termination.
	Timeout time.Duration
	// Status stores the last cached result of the health check function. Since the package uses pointers, this
	// original check can be referenced for the latest health information if desired without calling Check() on the
	// checker. This has the advantage of being able to get only the health information you need without any bubbling
	// up of status information by the other checks.
	Status Status
	// LastExecuted stores the last time the health check function was executed.
	LastExecuted time.Time
}

// NewCheck creates a new health check with suitable default values.
//
// TTL is set to a duration of 1 second (1 second cache between executions of the check function).
//
// Timeout is left at its default zero-value, meaning that there is no deadline for completion. It is recommended that
// you configure a timeout yourself but this is not required.
func NewCheck(name string, checkFunc func(ctx context.Context) Status) *Check {
	return &Check{
		Name: name,
		Func: checkFunc,
		TTL:  time.Second * 1,
	}
}

// Checker coordinates checks and executes their status functions to determine application health.
type Checker struct {
	// Checks contains all of the resources to monitor.
	Checks []*Check
}

// Start spins up a goroutine for each configured check that executes the check's check function and updates the
// check's status. The goroutine will wait between polls as defined by the check's TTL to avoid spamming the resource
// being evaluated. If a timeout is set on the check, the context provided to Start will be wrapped in a context with a
// deadline and provided to the check function to facilitate early termination.
func (chckr Checker) Start(ctx context.Context) {
	for _, check := range chckr.Checks {
		go func(check *Check) {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if check.Timeout > 0 {
						executeCheckWithTimeout(ctx, check)
					} else {
						executeCheck(ctx, check)
					}

					time.Sleep(check.TTL)
				}
			}
		}(check)
	}
}

// Check returns the latest cached status for all of the configured checks. While the return value is a pointer, it
// will never be nil.
//
// Individual check information can be pulled from the returned check statuses map, which uses the check's name as the
// key.
func (chckr Checker) Check() *CheckerStatus {
	checkStatuses := make(map[string]CheckStatus)

	// Use StateUp as the initial state so that it may be overidden by the check if necessary.
	// If checks are not configured, then we also default to StateUp.
	state := StateUp

	for _, check := range chckr.Checks {
		state = compareState(state, check.Status.State)
		checkStatuses[check.Name] = CheckStatus{
			Name:         check.Name,
			Status:       check.Status,
			LastExecuted: check.LastExecuted,
		}
	}

	return &CheckerStatus{State: state, CheckStatuses: checkStatuses}
}

// executeCheck executes the check function using the provided context and updates the check information.
func executeCheck(ctx context.Context, check *Check) {
	check.Status = check.Func(ctx)
	check.LastExecuted = time.Now()
}

// executeCheck executes the check function using the provided context, wrapped with a deadline set to the check's
// configured timeout, and updates the check information.
func executeCheckWithTimeout(ctx context.Context, check *Check) {
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, check.Timeout)
	defer cancelTimeout()

	executeCheck(timeoutCtx, check)
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
