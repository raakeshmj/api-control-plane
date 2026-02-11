package reliability

type FailureStrategy string

const (
	FailOpen   FailureStrategy = "fail_open"
	FailClosed FailureStrategy = "fail_closed"
)

// ShouldAllow determines if we should proceed given an error and a strategy
func ShouldAllow(strategy FailureStrategy, err error) bool {
	if err == nil {
		return true // No error, allow
	}

	if strategy == FailOpen {
		return true // Allow traffic despite error
	}

	return false // Block traffic
}
