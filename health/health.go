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

// CheckerStatus represents the health of the all of the resources being checked. The state combines all of the check
// states together and bubbles up the most degraded. For example, if the are three checks and one has a state of
// health.StateDown, the overall state will be health.StateDown.
type CheckerStatus struct {
	State         State
	CheckStatuses map[string]CheckStatus
}

// CheckStatus represents the health of the individual check.
type CheckStatus struct {
	Name         string
	Status       Status
	LastExecuted time.Time
}

// Status is a wrapper for resource health state and any additional, arbitrary details that you may wish to include.
type Status struct {
	State State
	// Details is for any additional information about the resource that you want to expose
	Details interface{}
}

// Check represents a resource to be checked. The provided check function is executed to determine health on a cadence,
// defined by the configured TTL. It is your responsibility to ensure that this function respects the provided context
// so that the logic may be terminated early. The configurable timeout plays into this as well by setting up the
// provided context with a deadline.
//
// Checker stores the most recently polled health information on the original check to cache the information.
type Check struct {
	Name         string                           // Check name
	Func         func(ctx context.Context) Status // Check function to be executed when determining health
	TTL          time.Duration                    // Time between health check execution
	Timeout      time.Duration                    // Max time that the check function may execute in
	Status       Status                           // Last status returned by the check function
	LastExecuted time.Time                        // Last time the check function completed
}

// NewCheck creates a new health check with suitable default values: TTL of 1 second, no timeouts.
func NewCheck(name string, checkFunc func(ctx context.Context) Status) *Check {
	return &Check{
		Name: name,
		Func: checkFunc,
		TTL:  time.Second * 1,
	}
}

// Checker coordinates checks and executes their status functions to determine application health.
type Checker struct {
	Checks []*Check // All of the checks to coordinate
}

// Start spins up a goroutine for each configured check that executes the check's check function and updates the
// check's status. The goroutine will wait between polls as defined by the check's TTL to avoid spamming the resource
// being evaluated. If a timeout is set on the check, the context provided to Start will be wrapped in a cancelable
// context and provided to the check function to facilitate early termination.
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

// Check returns the latest cached status for all of the configured checks. Individual check information can be pulled
// from the check statuses map, which uses the check's name as the key.
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

// compareState compares states and returns the most degraded state of the two
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
