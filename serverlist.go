package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"sort"
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
			continue
		}
		if SLine == "" || strings.HasPrefix(SLine, "#") {
			continue
		}
		if AddrPadding < len(SLine) {
			AddrPadding = len(SLine)
		}
		servers[section] = AppendUnique(servers[section], SLine)
	}
	return
}
