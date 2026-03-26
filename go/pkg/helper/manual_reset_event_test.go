package helper

import (
	"testing"
	"time"
)

func TestManualResetEventWaitSetReset(t *testing.T) {
	event := NewManualResetEvent(false)
	done := make(chan struct{})

	go func() {
		event.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Wait should block until Set is called")
	case <-time.After(20 * time.Millisecond):
	}

	event.Set()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Wait did not unblock after Set")
	}

	event.Reset()
	done = make(chan struct{})
	go func() {
		event.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Wait should block again after Reset")
	case <-time.After(20 * time.Millisecond):
	}

	event.Set()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Wait did not unblock after second Set")
	}
}
