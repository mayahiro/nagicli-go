// Command status demonstrates TTY-aware status reporting with plain-log fallback
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mayahiro/nagicli-go/status"
)

func main() {
	reporter := status.NewProcess()
	for tick := uint64(0); tick < 8; tick++ {
		if _, err := reporter.Update(status.NewSpinner(tick, "Preparing")); err != nil {
			fail(err)
		}
		time.Sleep(60 * time.Millisecond)
	}

	for completed := uint64(0); completed <= 4; completed++ {
		if _, err := reporter.Update(status.NewProgress(completed, 4, "Building")); err != nil {
			fail(err)
		}
		time.Sleep(60 * time.Millisecond)
	}
	if _, err := reporter.Finish(status.NewProgress(4, 4, "Complete")); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
