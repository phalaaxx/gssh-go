package main

import (
	"fmt"
	"github.com/phalaaxx/clap"
	"log"
	"math"
	"os"
	"sync"
	"time"
)

/* Global gssh version string */
var GsshVersion string = `gssh - group ssh, ver. 2.2
(c)2014-2025 Bozhin Zafirov <bozhin@deck17.com>
`

/* main program */
func main() {
	// local variables
	var err error

	// parse command line arguments
	OptUser := clap.String('u', "user", "root", "ssh login as this username [default: root]", false)
	OptFile := clap.String('f', "file", "", "file with the list of hosts", false)
	OptDelay := clap.Int('d', "delay", 100, "delay between each ssh fork (default 100 msec)", false)
	OptSection := clap.String('s', "section", "", "name of ini section containing servers list", false)
	OptProcesses := clap.Int('p', "processes", 500, "number of parallel ssh processes (default: 500)", false)
	OptNoStrict := clap.Bool('n', "no-strict", false, "don't use strict ssh fingerprint checking", false)
	OptAnsible := clap.Bool('a', "ansible-hosts", false, "Read ansible hosts file at /etc/ansible/hosts", false)
	OptTimeout := clap.Int('t', "timeout", 0, "kill ssh sessions running longer than this many seconds (default: no limit)", false)
	OptVersion := clap.Bool('v', "version", false, "Print version and exit", false)
	clap.Parse(true)

	/* print program version and exit */
	if *OptVersion {
		if _, err = os.Stderr.WriteString(GsshVersion); err != nil {
			log.Fatal(err)
		}
		os.Exit(1)
	}

	/* look for mandatory positional arguments */
	if clap.NArg() < 1 {
		log.Fatal("Nothing to do. Use -h for help.")
	}

	/* by default, read server list from stdin */
	ServerListFile := os.Stdin

	if *OptAnsible {
		*OptFile = "/etc/ansible/hosts"
	}

	/* read server names from file if a file name is supplied */
	if *OptFile != "" {
		ServerListFile, err = os.Open(*OptFile)
		if err != nil {
			log.Fatalf("ServerListFile: Error: %v", err)
		}
		ServerListFileClose := func() {
			if err := ServerListFile.Close(); err != nil {
				log.Println(err)
			}
		}
		defer ServerListFileClose()
	}
	AddrPadding, servers := LoadServerList(ServerListFile)

	srv := new(sync.WaitGroup)
	/* start output monitor goroutine */
	message, active := OutputMonitor(servers.Len(*OptSection), AddrPadding, srv)

	/* command to run on servers */
	OptCommand := clap.Arg(0)

	/* make new group */
	group := &SshGroup{
		Timeout: time.Duration(*OptTimeout) * time.Second,
	}

	/* no point to spawn more processes than servers */
	if *OptProcesses < 1 {
		log.Fatal("Number of parallel processes must be at least 1.")
	}
	*OptProcesses = int(math.Min(float64(*OptProcesses), float64(servers.Len(*OptSection))))

	/* limit the number of parallel ssh processes */
	slots := make(chan struct{}, *OptProcesses)

	/* print heading text */
	TemplateString := `%s
  [*] read (%d) hosts from the list
  [*] executing '%s' as user '%s'
  [*] spawning %d parallel ssh sessions

`
	if _, err = fmt.Fprintf(os.Stderr, TemplateString, GsshVersion, servers.Len(*OptSection), OptCommand, *OptUser, *OptProcesses); err != nil {
		log.Println(err)
	}

	/* spawn ssh processes */
	srv.Add(servers.Len(*OptSection) + 1)
	for section := range servers {
		if len(*OptSection) != 0 && section != *OptSection {
			/* skip current section */
			continue
		}
		for i, Server := range servers[section] {
			ssh := &SshServer{
				Username: *OptUser,
				Address:  Server,
			}
			group.Servers = append(group.Servers, ssh)
			/* wait for a free slot and run command */
			slots <- struct{}{}
			active <- 1
			go func() {
				defer func() { <-slots }()
				group.Command(ssh, OptCommand, *OptNoStrict, message, active, srv)
			}()
			/* time delay and max processes wait between spawns */
			if i < servers.Len(*OptSection) {
				time.Sleep(time.Duration(*OptDelay) * time.Millisecond)
			}
		}
	}
	/* wait for subprocesses to exit */
	srv.Wait()
}
