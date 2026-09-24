package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

/* ServerList defines a type for list of servers with sections */
type ServerList map[string]sort.StringSlice

/* Len returns the number of servers in the specified section */
func (s ServerList) Len(sectionName string) (count int) {
	for section := range s {
		if len(sectionName) == 0 || sectionName == section {
			count = count + len(s[section])
		}
	}
	return count
}

/* hostRange matches ansible style host ranges such as [01:10], [a:f] or [1:10:2] */
var hostRange = regexp.MustCompile(`\[([0-9]+|[a-z]):([0-9]+|[a-z])(?::([0-9]+))?\]`)

/* ExpandHostRange expands ansible style host ranges into a list of host names */
func ExpandHostRange(host string) (hosts []string) {
	loc := hostRange.FindStringSubmatchIndex(host)
	if loc == nil {
		return []string{host}
	}
	prefix, suffix := host[:loc[0]], host[loc[1]:]
	first, last := host[loc[2]:loc[3]], host[loc[4]:loc[5]]
	step := 1
	if loc[6] >= 0 {
		step, _ = strconv.Atoi(host[loc[6]:loc[7]])
	}
	if step < 1 {
		return []string{host}
	}
	var items []string
	start, errStart := strconv.Atoi(first)
	end, errEnd := strconv.Atoi(last)
	switch {
	case errStart == nil && errEnd == nil:
		/* numeric range, zero padded to the width of the first value */
		width := 0
		if len(first) > 1 && first[0] == '0' {
			width = len(first)
		}
		for i := start; i <= end; i += step {
			items = append(items, fmt.Sprintf("%0*d", width, i))
		}
	case errStart != nil && errEnd != nil:
		/* alphabetic range */
		for c := first[0]; c <= last[0]; c += byte(step) {
			items = append(items, string(c))
			if int(c)+step > 'z' {
				break
			}
		}
	default:
		return []string{host}
	}
	/* expand any remaining ranges in the suffix */
	for _, item := range items {
		for _, rest := range ExpandHostRange(suffix) {
			hosts = append(hosts, prefix+item+rest)
		}
	}
	return hosts
}

/* LoadServerList loads a list of server addresses from a file */
func LoadServerList(file *os.File) (AddrPadding int, servers ServerList) {
	servers = make(map[string]sort.StringSlice)
	AppendUnique := func(sectionList sort.StringSlice, Server string) []string {
		if !sort.StringsAreSorted(sectionList) {
			sort.Strings(sectionList)
		}
		idx := sort.SearchStrings(sectionList, Server)
		if idx < len(sectionList) && sectionList[idx] == Server {
			return sectionList
		}
		return append(sectionList[:idx], append(sort.StringSlice{Server}, sectionList[idx:]...)...)
	}
	Reader := bufio.NewReader(file)
	section := "main"
	skipSection := false
	for {
		Line, err := Reader.ReadString('\n')
		if err != nil && err != io.EOF {
			log.Fatalf("LoadServerList: Error: %v", err)
		}
		/* the last line may not end with a newline */
		if err == io.EOF && Line == "" {
			break
		}
		SLine := strings.TrimSpace(Line)
		if strings.HasPrefix(SLine, "[") && strings.HasSuffix(SLine, "]") {
			section = SLine[1 : len(SLine)-1]
			/* ansible [group:vars] and [group:children] sections do not list hosts */
			skipSection = strings.Contains(section, ":")
			continue
		}
		if SLine == "" || strings.HasPrefix(SLine, "#") || strings.HasPrefix(SLine, ";") || skipSection {
			continue
		}
		/* ignore host variables, e.g. "host1 ansible_host=10.0.0.1" */
		for _, Server := range ExpandHostRange(strings.Fields(SLine)[0]) {
			if AddrPadding < len(Server) {
				AddrPadding = len(Server)
			}
			servers[section] = AppendUnique(servers[section], Server)
		}
	}
	return
}
