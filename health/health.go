package health

import (
	"context"
	"time"
)

// State represents the health of the resource being checked as a simple value
type State int

// States must be ordered from most degraded to least degraded here so that checks work their way up to the best state

const (
	StateDown State = iota
	StateDegraded
	StateUp
)

// CheckerStatus represents the health of the all of the resources being checked
type CheckerStatus struct {
	State         State                  // Overall state
	CheckStatuses map[string]CheckStatus // Statuses for each of the checks
}

// CheckStatus represents the health of the check
type CheckStatus struct {
	Name         string    // Check name
	Status       Status    // Check status
	LastExecuted time.Time // Last time the check completed
}

// Status represents resource health
type Status struct {
	State   State       // Health state for the resource
	Details interface{} // Any additional information about the resource that you want to expose
}

// Check represents a resource to be checked
type Check struct {
	Name         string                           // Name of the check
	Func         func(ctx context.Context) Status // Check function to be executed when determining health. If a timeout is configured, the provided context will have an associated deadline that must be handled.
	TTL          time.Duration                    // Time between health check execution
	Timeout      time.Duration                    // Max time that the check function may execute in, automatically set on the context for the check function
	Status       Status                           // Last status returned by the check function
	LastExecuted time.Time                        // Last time the check function completed
}

// NewCheck creates a new health check with suitable default values (TTL of 1 second, no timeouts)
func NewCheck(name string, checkFunc func(ctx context.Context) Status) *Check {
	return &Check{
		Name: name,
		Func: checkFunc,
		TTL:  time.Second * 1,
	}
}

// Checker coordinates checks and executes their status functions to determine application health.
type Checker struct {
	Checks []*Check // All of the checks to be checked
}

// Start spawns a goroutine for each configured check that updates the check's status according to the configured
// timeout. To stop executing all of the checks, cancel the provided context.
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

// Check returns the latest cached status for all of the configured checks
func (chkr Checker) Check() *CheckerStatus {
	checkStatuses := make(map[string]CheckStatus)

	// Use Up as the initial state so that it may be overriden by the check if necessary
	// If checks are not configured, then we also default to Up
	state := StateUp

	for _, check := range chkr.Checks {
		state = compareState(state, check.Status.State)
		checkStatuses[check.Name] = CheckStatus{Name: check.Name, Status: check.Status, LastExecuted: check.LastExecuted}
	}

	return &CheckerStatus{State: state, CheckStatuses: checkStatuses}
}

// executeCheck executes the check function and updates the check information
func executeCheck(ctx context.Context, check *Check) {
	check.Status = check.Func(ctx)
	check.LastExecuted = time.Now()
}

// executeCheck executes the check function and updates the check information using the check's configured timeout
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
	} else if stateA == StateDegraded || stateB == StateDegraded {
		state = StateDegraded
	} else {
		state = StateUp
	}

	return state
}
