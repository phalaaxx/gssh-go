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
	OptNoStrict := clap.Bool('n', "no-strict", false, "accept fingerprints of new hosts, still refuse changed ones", false)
	OptForwardAgent := clap.Bool('A', "forward-agent", false, "enable ssh agent forwarding", false)
	OptAnsible := clap.Bool('a', "ansible-hosts", false, "Read ansible hosts file at /etc/ansible/hosts", false)
	OptTimeout := clap.Int('t', "timeout", 0, "kill ssh sessions running longer than this many seconds (default: no limit)", false)
	OptVersion := clap.Bool('v', "version", false, "Print version and exit", false)
	clap.Parse(true)

	/* print program version and exit */
	if *OptVersion {
		if _, err = os.Stderr.WriteString(GsshVersion); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
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
	} else if IsTerminal(os.Stdin) {
		log.Fatal("No list of servers. Use -f or supply the list to standard input.")
	}
	servers := LoadServerList(ServerListFile)
	if _, ok := servers[*OptSection]; *OptSection != "" && !ok {
		log.Fatalf("Section '%s' not found in the list of servers.", *OptSection)
	}

	/* run the command only once on servers listed in multiple sections */
	hosts := servers.Hosts(*OptSection)
	AddrPadding := 0
	for _, host := range hosts {
		if AddrPadding < len(host) {
			AddrPadding = len(host)
		}
	}

	srv := new(sync.WaitGroup)
	/* start output monitor goroutine */
	message, active, failed := OutputMonitor(len(hosts), AddrPadding, srv)

	/* command to run on servers */
	OptCommand := clap.Arg(0)

	/* make new group */
	group := &SshGroup{
		Timeout:      time.Duration(*OptTimeout) * time.Second,
		ForwardAgent: *OptForwardAgent,
	}

	/* no point to spawn more processes than servers */
	if *OptProcesses < 1 {
		log.Fatal("Number of parallel processes must be at least 1.")
	}
	*OptProcesses = int(math.Min(float64(*OptProcesses), float64(len(hosts))))

	/* limit the number of parallel ssh processes */
	slots := make(chan struct{}, *OptProcesses)

	/* print heading text */
	TemplateString := `%s
  [*] read (%d) hosts from the list
  [*] executing '%s' as user '%s'
  [*] spawning %d parallel ssh sessions

`
	if _, err = fmt.Fprintf(os.Stderr, TemplateString, GsshVersion, len(hosts), OptCommand, *OptUser, *OptProcesses); err != nil {
		log.Println(err)
	}

	/* spawn ssh processes */
	srv.Add(len(hosts))
	for i, Server := range hosts {
		ssh := &SshServer{
			Username: *OptUser,
			Address:  Server,
		}
		group.Servers = append(group.Servers, ssh)
		/* wait for a free slot and run command */
		slots <- struct{}{}
		active <- Status{Delta: 1}
		go func() {
			defer func() { <-slots }()
			group.Command(ssh, OptCommand, *OptNoStrict, message, active, srv)
		}()
		/* time delay between spawns, except after the last one */
		if i < len(hosts)-1 {
			time.Sleep(time.Duration(*OptDelay) * time.Millisecond)
		}
	}
	/* wait for subprocesses to exit */
	srv.Wait()

	/* exit with an error if the command failed on any server */
	if *failed > 0 {
		os.Exit(1)
	}
}
