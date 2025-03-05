package shutdown

import (
	"sync"
	"testing"
	"time"
)

// TestShutdown verifies that the shutdown signal is properly sent and received
// by a single listener.
func TestShutdown(t *testing.T) {
	shutdown := New()

	wg := sync.WaitGroup{}

	// Start a goroutine that waits for the shutdown signal.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait for the shutdown signal.
		<-shutdown.Done()
	}()

	// Simulate some work before triggering shutdown.
	time.Sleep(100 * time.Millisecond)
	// Trigger the shutdown.
	shutdown.Do()
	// Wait for the goroutine to finish.
	wg.Wait()
}

// TestShutdown_TwoListener verifies that multiple listeners can receive the
// shutdown signal simultaneously.
func TestShutdown_TwoListener(t *testing.T) {
	shutdown := New()

	wg := sync.WaitGroup{}

	// Start the first goroutine that waits for the shutdown signal.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait for the shutdown signal.
		<-shutdown.Done()
	}()

	// Start the second goroutine that also waits for the shutdown signal.
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Wait for the shutdown signal.
		<-shutdown.Done()
	}()

	// Simulate some work before triggering shutdown.
	time.Sleep(100 * time.Millisecond)
	// Trigger the shutdown.
	shutdown.Do()
	// Wait for the goroutine to finish.
	wg.Wait()
}
