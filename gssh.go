package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"
)

/* Global gssh version string */
var GsshVersion = `gssh - group ssh, ver. 2.1
(c)2014-2025 Bozhin Zafirov <bozhin@deck17.com>
`

/* main program */
func main() {
	// local variables
	var err error

	// parse command line arguments
	OptUser := flag.String("u", "root", "ssh login as this username")
	OptFile := flag.String("f", "", "file with the list of hosts")
	OptDelay := flag.Int("d", 100, "delay between each ssh fork (default 100 msec)")
	OptSection := flag.String("s", "", "name of ini section containing servers list")
	OptProcesses := flag.Int("p", 500, "number of parallel ssh processes (default: 500)")
	OptNoStrict := flag.Bool("n", false, "don't use strict ssh fingerprint checking")
	OptAnsible := flag.Bool("a", false, "Read ansible hosts file at /etc/ansible/hosts")
	OptVersion := flag.Bool("v", false, "Print version and exit")
	OptHelp := flag.Bool("h", false, "show this help screen")
	flag.Parse()

	/* show help screen and exit in case of -h or --help option */
	if *OptHelp {
		flag.Usage()
		os.Exit(1)
	}

	/* print program version and exit */
	if *OptVersion {
		if _, err = fmt.Fprintf(os.Stderr, GsshVersion); err != nil {
			log.Fatal(err)
		}
		os.Exit(1)
	}

	/* look for mandatory positional arguments */
	if flag.NArg() < 1 {
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
			log.Fatal(fmt.Sprintf("ServerListFile: Error: %v", err))
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
	OptCommand := flag.Arg(0)

	/* make new group */
	group := new(SshGroup)

	/* no point to display more processes than */
	*OptProcesses = int(math.Max(float64(*OptProcesses), float64(servers.Len(*OptSection))))

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
			/* run command */
			active <- 1
			go group.Command(ssh, OptCommand, *OptNoStrict, message, active, srv)
			/* time delay and max processes wait between spawns */
			if i < servers.Len(*OptSection) {
				time.Sleep(time.Duration(*OptDelay) * time.Millisecond)
			}
		}
	}
	/* wait for subprocesses to exit */
	srv.Wait()
}
