package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
)

/* SshServer connection data */
type SshServer struct {
	Username string
	Address  string
}

/* SshGroup client group */
type SshGroup struct {
	Servers []*SshServer
}

/* Command runs a new ssh session to the specified server and prints output from command sent to the server */
func (s *SshGroup) Command(ssh *SshServer, Command string, NoStrict bool, message chan Message, active chan int, srv *sync.WaitGroup) {
	defer func() {
		active <- -1
		srv.Done()
	}()

	/* host key checking from commandline arguments */
	StrictHostKeyChecking := "StrictHostKeyChecking=yes"
	if NoStrict {
		StrictHostKeyChecking = "StrictHostKeyChecking=no"
	}

	cmd := exec.Command("env",
		"ssh",
		"-A",
		"-o", "PasswordAuthentication=no",
		"-o", StrictHostKeyChecking,
		"-o", "GSSAPIAuthentication=no",
		"-o", "HostbasedAuthentication=no",
		"-l", ssh.Username,
		ssh.Address,
		Command)

	/* Failed reports an error which prevented the ssh session from running */
	Failed := func(err error) {
		message <- Message{
			Server: ssh.Address,
			Data:   fmt.Sprintf("gssh: %v\n", err),
			Stdout: false,
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		Failed(err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		Failed(err)
		return
	}

	/* define Stdout and Stderr read buffers */
	Stdout := bufio.NewReader(stdout)
	Stderr := bufio.NewReader(stderr)

	/* run the command */
	if err := cmd.Start(); err != nil {
		Failed(err)
		return
	}

	var w sync.WaitGroup
	w.Add(2)

	PrintOutput := func(stdout bool, Std *bufio.Reader) {
		defer w.Done()
		for {
			line, err := Std.ReadString('\n')
			if err == io.EOF && line == "" {
				break
			}
			if err != nil && err != io.EOF {
				log.Printf("PrintOutput: %s: Error: %v\n", ssh.Address, err)
				break
			}
			/* the last line of output may not end with a newline */
			if !strings.HasSuffix(line, "\n") {
				line += "\n"
			}
			message <- Message{
				Server: ssh.Address,
				Data:   line,
				Stdout: stdout,
			}
		}
	}

	go PrintOutput(true, Stdout)
	go PrintOutput(false, Stderr)

	w.Wait()
	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			log.Println(err)
		}
	}
}
