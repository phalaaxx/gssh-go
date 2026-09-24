package main

import (
	"fmt"
	"log"
	"os"
	"sync"
)

/* Message defines an output message for OutputMonitor */
type Message struct {
	Server string
	Data   string
	Stdout bool
}

/* Status reports a started (Delta 1) or a finished (Delta -1) ssh session */
type Status struct {
	Delta  int
	Failed bool
}

/* OutputMonitor prints out stats and server data to screen */
func OutputMonitor(total int, padding int, srv *sync.WaitGroup) (chan Message, chan Status, *int) {
	var cntActive int
	var cntComplete int
	var cntFailed int
	var Template string
	var OutDev *os.File

	message := make(chan Message)
	active := make(chan Status)

	type OutputStats map[string]bool

	StdoutStats := make(OutputStats)
	StderrStats := make(OutputStats)

	/* progress is printed to stderr, so only show it when stderr is a terminal */
	ShowProgress := IsTerminal(os.Stderr)

	/* initialize output template strings */
	OutTemplate := "%*s%s \033[01;32m->\033[0m %s"
	ErrTemplate := "%*s%s \033[01;31m=>\033[0m %s"
	OutArrow := "\033[01;32m->\033[0m"
	ErrArrow := "\033[01;31m=>\033[0m"

	/* disable colored output in case output is redirected */
	if !IsTerminal(os.Stdout) {
		OutTemplate = "%*s%s -> %s"
	}
	if !ShowProgress {
		ErrTemplate = "%*s%s => %s"
		OutArrow = "->"
		ErrArrow = "=>"
	}

	/* ClearProgress defines a local function to clear command progress */
	ProgressLen := 0
	ClearProgress := func() {
		if !ShowProgress || ProgressLen == 0 {
			return
		}
		if _, err := fmt.Fprintf(os.Stderr, "\r%*s\r", ProgressLen, ""); err != nil {
			log.Println(err)
		}
		ProgressLen = 0
	}
	PrintProgress := func() {
		if !ShowProgress {
			return
		}
		n, err := fmt.Fprintf(os.Stderr, "[%d/%d] %.2f%% complete, %d active",
			cntComplete,
			total,
			float64(cntComplete)*float64(100)/float64(total),
			cntActive,
		)
		if err != nil {
			log.Println(err)
		}
		ProgressLen = n
	}

	/* statistics variables */
	var StdoutServersCount int
	var StderrServersCount int
	var StdoutLinesCount int
	var StderrLinesCount int

	OutputCallback := func(message chan Message, active chan Status, srv *sync.WaitGroup) {
		defer srv.Done()
		for cntComplete != total {
			select {
			case msg := <-message:
				if msg.Stdout {
					Template = OutTemplate
					if _, ok := StdoutStats[msg.Server]; !ok {
						StdoutStats[msg.Server] = true
						StdoutServersCount++
					}
					StdoutLinesCount++
					OutDev = os.Stdout
				} else {
					Template = ErrTemplate
					if _, ok := StderrStats[msg.Server]; !ok {
						StderrStats[msg.Server] = true
						StderrServersCount++
					}
					StderrLinesCount++
					OutDev = os.Stderr
				}
				ClearProgress()
				if _, err := fmt.Fprintf(OutDev, Template, padding-len(msg.Server)+1, " ", msg.Server, msg.Data); err != nil {
					log.Println(err)
				}
				PrintProgress()
			case status := <-active:
				/* update active count and exit if cntActive is zero */
				if status.Delta < 0 {
					cntComplete = cntComplete - status.Delta
				}
				if status.Failed {
					cntFailed++
				}
				cntActive = cntActive + status.Delta
				ClearProgress()
				PrintProgress()
			}
		}
		/* calculate and print end stats */
		ClearProgress()
		_, err := fmt.Fprintf(os.Stderr,
			"\n  Done. Processed: %d / Output: %d (%d) / %s %d (%d) / %s %d (%d) / Failed: %d\n",
			total,
			StdoutServersCount+StderrServersCount,
			StdoutLinesCount+StderrLinesCount,
			OutArrow,
			StdoutServersCount,
			StdoutLinesCount,
			ErrArrow,
			StderrServersCount,
			StderrLinesCount,
			cntFailed,
		)
		if err != nil {
			log.Println(err)
		}
	}
	srv.Add(1)
	go OutputCallback(message, active, srv)

	return message, active, &cntFailed
}
