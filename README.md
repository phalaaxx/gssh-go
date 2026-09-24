gssh
----

Run ssh command on a group of servers simultaneously.
This project was inspired from *mpssh* and is written in Go. The reason for another mpssh fork is that it is a fun thing to do with Go.


Requirements
------------

In order to use gssh, the ssh binary from openssh package must be installed in user's path.
Also the machine running gssh should be able to connect to every server listed in the file with hosts without a password - either with a passwordless key or with ssh agent. 

Build
-----

To build gssh with official golang compiler, use the following command:

	go build
	./gssh -h

Another option is to use make to build with the sample makefile:

	make
	make install PREFIX=/usr

Usage
-----

A list of servers is mandatory to use gssh. The list is a plain text file with one server at a line (no username):

	cat << EOF > servers.txt
	server1.domain.tld
	server2.domain2.tld
	1.2.3.4
	EOF


To actually run a command on all files from the list:

	./gssh -f servers.txt 'uptime'


Alternative method to run gssh is to supply list of servers to standard input:

	cat << EOF | gssh 'uptime'
	server1.domain.tld
	server2.domain2.tld
	1.2.3.4
	EOF
	

Or to cat list files:

	cat servers.txt servers2.txt | gssh 'uname -r'


A full list of currently supported arguments can be obtained with the -h option:

	$ gssh -h
	Usage: gssh [OPTIONS]

	Options:
	  -u, --user <USER>            ssh login as this username [default: root]
	  -f, --file <FILE>            file with the list of hosts
	  -d, --delay <DELAY>          delay between each ssh fork (default 100 msec)
	  -s, --section <SECTION>      name of ini section containing servers list
	  -p, --processes <PROCESSES>  number of parallel ssh processes (default: 500)
	  -n, --no-strict              accept fingerprints of new hosts, still refuse changed ones
	  -A, --forward-agent          enable ssh agent forwarding
	  -a, --ansible-hosts          Read ansible hosts file at /etc/ansible/hosts
	  -t, --timeout <TIMEOUT>      kill ssh sessions running longer than this many seconds (default: no limit)
	  -v, --version                Print version and exit

Options:

  * **d** - this is the time in miliseconds to wait between spawning next process
  * **f** - name of a text file containing list of servers; lines starting with # and empty lines are ignored
  * **p** - maximum number of ssh processes running at the same time
  * **n** - accept host keys of servers which are not yet in known_hosts (StrictHostKeyChecking=accept-new); default is to use strict checking
  * **A** - forward the ssh agent to the servers; disabled by default
  * **a** - read the list of servers from the ansible inventory at /etc/ansible/hosts; host variables and [group:vars] / [group:children] sections are ignored and host ranges such as web[01:10] are expanded
  * **t** - kill ssh sessions which run longer than this many seconds; 0 (default) means no limit
  * **u** - username to use for ssh login
  * **s** - name of ini-like section in input file under which is the list of servers to process; without it servers from all sections are used, each server only once

ssh always runs in batch mode, so it never asks for passwords or passphrases, with a 10 seconds connect timeout.

gssh exits with status 1 if the command failed on any server: ssh could not connect, the command timed out or it exited with a non-zero status.
