package logger

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

// TestRaceConditionBug tests that the race condition described in the issue is fixed.
// Without the mutex, ANSI escape sequences from multiple goroutines can interleave,
// causing text to appear in the wrong color.
// The main validation is done by running with -race flag to detect any data races.
func TestRaceConditionBug(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		Stdout: &buf,
		Stderr: &buf,
		Color:  true, // Even if colors are disabled in test env, we test the sync behavior
	}

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Simulate the scenario: multiple goroutines writing colored output simultaneously
	// Goroutine 1 writes green, Goroutine 2 writes magenta
	// Without mutex: escape codes can interleave making text appear in wrong colors
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			if id%2 == 0 {
				// Green output (would be ANSI: \033[32m in color-enabled terminal)
				logger.Outf(Green, "green message %d\n", id)
			} else {
				// Magenta output (would be ANSI: \033[35m in color-enabled terminal)
				logger.Outf(Magenta, "magenta message %d\n", id)
			}
		}(i)
	}

	wg.Wait()

	output := buf.String()

	// Verify we got output
	if len(output) == 0 {
		t.Fatal("Expected output but got none")
	}

	// The primary goal is to ensure no race conditions occur.
	// With the mutex in place, this test should pass reliably with -race flag.
	// We verify basic structure: output contains both green and magenta messages
	if !strings.Contains(output, "green message") {
		t.Error("Expected to find 'green message' in output")
	}
	if !strings.Contains(output, "magenta message") {
		t.Error("Expected to find 'magenta message' in output")
	}
}

// TestConcurrentOutput tests that concurrent writes to the logger don't cause race conditions
func TestConcurrentOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		Stdout: &buf,
		Stderr: &buf,
		Color:  true,
	}

	const numGoroutines = 100
	const numIterations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // *2 because we test both Outf and Errf

	// Test concurrent Outf calls
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				logger.Outf(Green, "goroutine %d iteration %d\n", id, j)
			}
		}(i)
	}

	// Test concurrent Errf calls
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				logger.Errf(Red, "error from goroutine %d iteration %d\n", id, j)
			}
		}(i)
	}

	wg.Wait()

	// If we got here without a race condition, the test passes
	// We just need to verify that we got some output
	if buf.Len() == 0 {
		t.Error("Expected output but got none")
	}
}

// TestConcurrentFOutf tests that concurrent writes to FOutf don't cause race conditions
func TestConcurrentFOutf(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		Stdout: &buf,
		Stderr: &buf,
		Color:  true,
	}

	const numGoroutines = 50
	const numIterations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				logger.FOutf(&buf, Magenta, "FOutf from goroutine %d iteration %d\n", id, j)
			}
		}(i)
	}

	wg.Wait()

	// If we got here without a race condition, the test passes
	if buf.Len() == 0 {
		t.Error("Expected output but got none")
	}
}

// TestConcurrentMixedColors tests that concurrent writes with different colors don't interleave
func TestConcurrentMixedColors(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		Stdout: &buf,
		Stderr: &buf,
		Color:  true,
	}

	const numGoroutines = 20
	colors := []Color{Green, Red, Blue, Yellow, Magenta, Cyan}

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			color := colors[id%len(colors)]
			logger.Outf(color, "message from goroutine %d\n", id)
		}(i)
	}

	wg.Wait()

	// The test passes if we don't have a race condition
	if buf.Len() == 0 {
		t.Error("Expected output but got none")
	}
}

func TestResetLineStart(t *testing.T) {
	t.Parallel()

	t.Run("prepends carriage return for terminal lines", func(t *testing.T) {
		t.Parallel()
		if got := resetLineStart("task: hello\n", true); got != "\rtask: hello\n" {
			t.Fatalf("expected carriage return prefix, got %q", got)
		}
	})

	t.Run("does not change non-terminal output", func(t *testing.T) {
		t.Parallel()
		if got := resetLineStart("task: hello\n", false); got != "task: hello\n" {
			t.Fatalf("expected unchanged string, got %q", got)
		}
	})

	t.Run("does not change partial lines", func(t *testing.T) {
		t.Parallel()
		if got := resetLineStart("task: hello", true); got != "task: hello" {
			t.Fatalf("expected unchanged partial line, got %q", got)
		}
	})

	t.Run("does not double prepend carriage return", func(t *testing.T) {
		t.Parallel()
		if got := resetLineStart("\rtask: hello\n", true); got != "\rtask: hello\n" {
			t.Fatalf("expected unchanged carriage return line, got %q", got)
		}
	})
}
