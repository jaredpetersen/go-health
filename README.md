# go-health
#### 🏥 Barebones, detailed health check library for Go
go-health does away with the kitchen sink mentality of other health check libraries. You aren't getting a default HTTP
handler out of the box that is router dependent or has opinions about the shape or format of the health data being
published. You aren't getting pre-built health checks. But you do get a simple system for checking the health of
resources asynchronously with built-in caching and timeouts. Only what you absolutely need, and nothing else.

## Quickstart
```go
// Create your health checks
redisHealthCheckFunc := func(ctx context.Context) health.Status {
    return health.Status{State: health.StateUp}
}
redisHealthCheck := NewCheck("redis", redisHealthCheckFunc)

cockroachDBHealthCheckFunc := func(ctx context.Context) health.Status {
    return health.Status{State: health.StateUp}
}
cockroachDbHealthCheck := NewCheck("cockroachDb", cockroachDBHealthCheckFunc)
cockroachDbHealthCheck.Timeout = time.Second * 2

// Add the checks to a slice for consumption
healthChecks := []*health.Check{redisHealthCheck, cockroachDbHealthCheck}

// Set up the health checker that will be executing these checks
ctx := context.Background()
healthChecker := health.Checker{Checks: checks}

// Kick off a goroutine for each check automatically and store the results on the original check
healthChecker.Start(ctx)

// Retrieve the most recent result for all of the checks
healthCheckerStatus := healthChecker.Check()
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


```go
exampleCheckFunc := func(ctx context.Context) health.Status {
    statusDown := health.Status{State: health.StateDown}

    req, err := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)
    if err != nil {
        return statusDown
    }

    client := http.Client{}
    res, err := client.Do(req)
    if err != nil {
        return statusDown
    }

    if res.StatusCode == http.StatusOK {
        return health.Status{State: health.StateUp}
    } else {
        return statusDown
    }
}
exampleCheck := NewCheck("example", exampleCheckFunc)
exampleCheck.Timeout = time.Millisecond * 400

ctx := context.Background()

healthChecker := Checker{Checks: []*Check{exampleCheck}}
healthChecker.Start(ctx)
```

## Additional Information
The return type of the health check function supports adding arbitrary information to the status. This could be
information like active database connections, response time for an HTTP request, etc.

```go
type CustomHTTPStatusDetails struct {
    ResponseTime time.Duration
}
```

```go
return health.Status{
    State:   health.StateUp,
    Details: CustomHTTPStatusDetails{ResponseTime: time.Millisecond * 352},
}
