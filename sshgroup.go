package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

/* SshServer connection data */
type SshServer struct {
	Username string
	Address  string
}

/* SshGroup client group */
type SshGroup struct {
	Servers      []*SshServer
	Timeout      time.Duration
	ForwardAgent bool
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
		StrictHostKeyChecking = "StrictHostKeyChecking=accept-new"
	}

	/* ssh agent forwarding from commandline arguments */
	ForwardAgent := "ForwardAgent=no"
	if s.ForwardAgent {
		ForwardAgent = "ForwardAgent=yes"
	}

	/* limit the total run time of the command if requested */
	ctx := context.Background()
	if s.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "env",
		"ssh",
		"-o", ForwardAgent,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
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

	/* on timeout also close the pipes, in case a child of ssh keeps them open */
	cmd.Cancel = func() error {
		err := cmd.Process.Kill()
		_ = stdout.Close()
		_ = stderr.Close()
		return err
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
				if ctx.Err() != nil {
					/* pipes were closed on timeout */
					break
				}
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
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		Failed(fmt.Errorf("command timed out after %v", s.Timeout))
		return
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			log.Println(err)
		}
	}
}
