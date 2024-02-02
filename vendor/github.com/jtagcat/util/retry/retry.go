package retry

import (
	"context"

	"k8s.io/apimachinery/pkg/util/wait"
)

// Do copy this code instead of depending on this library
//
// similar to "k8s.io/apimachinery/pkg/util/retry"
func OnError(backoff wait.Backoff, fn func() (retryable bool, _ error)) error {
	return wait.ExponentialBackoff(backoff, func() (done bool, _ error) {
		retryable, err := fn()
		if err == nil || !retryable {
			return true, nil
		}
		return false, nil
	})
}

// using this requires the following in your go.mod:
//
//	// for github.com/jtagcat/util/retry
//	replace k8s.io/apimachinery => github.com/jtagcat/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20221027124836-581f57977fff
//
// error is only returned by context
func OnErrorManagedBackoff(ctx context.Context, backoff wait.Backoff, fn func() (retryable bool, _ error)) error {
	return wait.ManagedExponentialBackoffWithContext(ctx, backoff, func() (done bool, _ error) {
		retryable, err := fn()
		if err == nil || !retryable {
			return true, nil
		}
		return false, nil
	})
}
