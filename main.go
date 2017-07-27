package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sync"
)

var (
	npar    = flag.Int("n", 1, "Maximum number of jobs to run in parallel.")
	joblog  = flag.Bool("l", false, "Write stdout and stderr of each job to a file in the current directory.")
	verbose = flag.Bool("v", false, "Write status information to stdout.")
)

const usage = `Usage: %s [options] [file]

Execute jobs in batches.

Jobs are read from [file] or stdin, and will be executed by $SHELL.

Options:
`

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, usage, os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	log.SetPrefix("batch: ")
	log.SetFlags(0)
	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	in := os.Stdin
	if flag.Arg(0) != "" {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			log.Fatal(err)
		}
		in = f
	}

	var (
		scanner = bufio.NewScanner(in)
		sema    = make(chan int, *npar)
		wg      = new(sync.WaitGroup)
	)
	for n := 0; true; n++ {
		if !scanner.Scan() {
			break
		}
		sema <- 1
		wg.Add(1)
		job := scanner.Text()
		log.Printf("starting job [%d]: %s", n, job)
		go func(n int) {
			defer wg.Done()
			run(job, n)
			<-sema
		}(n)
	}

	wg.Wait()

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}

func run(job string, i int) {
	cmd := exec.Command(os.Getenv("SHELL"))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[%d] %v", i, err)
		return
	}
	go func() {
		stdin.Write([]byte(job))
		stdin.Close()
	}()

	if *joblog {
		f, err := os.Create(fmt.Sprintf("%d.out", i))
		if err != nil {
			log.Println(err)
		} else {
			defer f.Close()
			cmd.Stdout = f
			cmd.Stderr = f
		}
	}

	if err := cmd.Run(); err != nil {
		log.Printf("[%d] %v", i, err)
		return
	}
	log.Printf("[%d]: success=%v", i, cmd.ProcessState.Success())
}
