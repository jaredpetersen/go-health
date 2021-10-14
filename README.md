# go-health
#### 🏥 Barebones, detailed health check library for Go
go-health does away with the kitchen sink mentality of other health check libraries. You aren't getting a default HTTP
handler out of the box that is router dependent or has opinions about the shape or format of the health data being
published. You aren't getting pre-built health checks. But you do get a simple system for checking the health of
resources asynchronously with built-in caching and timeouts. Only what you absolutely need, and nothing else.

## Quickstart
```go
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
```

## Asynchronous Checking and Caching
Health checks in this package are only asynchronous for security purposes. Health endpoints in HTTP services can be a
lucrative target for bad actors looking to execute a Denial of Service (DOS) attack. Some checks that your application
performs to determine health may be somewhat expensive and a health endpoint centralizes all of those checks in a
single convenient location for an attacker. This attacker can conveniently direct significant amount of traffic to your
health check endpoints in an attempt to abuse and take down multiple systems in one go.

The best defense against this is to perform those checks on your schedule asynchronously, not the caller's schedule.
Results from those checks should be cached and returned upon request.

This package spins up a goroutine for each configured health check that polls each check function. Each check has its
own, individually configurable Time To Live (TTL) that dictates the duration between those polls. For example, if you
specify a check with a TTL of two seconds, the check will execute, wait two seconds, and then execute again and again
until the context is closed.

By default, all checks created via `health.NewCheck()` are configured with a default TTL of one second.

## Timeouts
You can optionally configure a timeout for each check. If set, the context provided to the function defined on the
check will have a deadline set. When the deadline expires, the context will close the Done channel, just like the
normal context behavior. go-health does not automatically kill the check execution, it only leverages the context
to communicate that the configured timeout deadline has been exceeded. It is your responsibility to handle the context
appropriately. For more information on context with deadline, see the
[context documentation](https://pkg.go.dev/context#WithDeadline).

## Additional Information
The return type of the health check function supports adding arbitrary information to the status. This could be
information like active database connections, response time for an HTTP request, etc.

```go
type HTTPHealthCheckDetails struct {
    ResponseTime time.Duration
}
```

```go
return health.Status{
    State:   health.StateUp,
    Details: HTTPHealthCheckDetails{ResponseTime: responseTime},
}
```
