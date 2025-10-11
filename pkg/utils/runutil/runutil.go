package runutil

import "time"

// Retry executes f every interval until timeout or on error is returned from f
func Retry(interval time.Duration, stopc <-chan struct{}, f func() error) error {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		err := f()
		if err == nil {
			return nil
		}

		select {
		case <-tick.C:
		case <-stopc:
			return err
		}
	}
}
