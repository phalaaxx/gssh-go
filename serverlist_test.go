package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExpandHostRange(t *testing.T) {
	tests := map[string][]string{
		"host1.example.com":     {"host1.example.com"},
		"web[1:3]":              {"web1", "web2", "web3"},
		"web[01:03].local":      {"web01.local", "web02.local", "web03.local"},
		"web[1:10:4]":           {"web1", "web5", "web9"},
		"db-[a:c]":              {"db-a", "db-b", "db-c"},
		"db-[a:b]-[1:2]":        {"db-a-1", "db-a-2", "db-b-1", "db-b-2"},
		"web[3:1]":              nil,
		"web[1:b]":              {"web[1:b]"},
		"web[1:3:0]":            {"web[1:3:0]"},
		"[2001:db8::1]":         {"[2001:db8::1]"},
		"host[x:z].example.com": {"hostx.example.com", "hosty.example.com", "hostz.example.com"},
	}
	for host, expected := range tests {
		if result := ExpandHostRange(host); !reflect.DeepEqual(result, expected) {
			t.Errorf("ExpandHostRange(%q) = %q, expected %q", host, result, expected)
		}
	}
}

func TestLoadServerList(t *testing.T) {
	data := `host0
# comment
[web]
web[1:2] ansible_host=10.0.0.1
web1

[web:vars]
ansible_user=admin

[all:children]
web
[db]
db1 # inline comment
db2`
	name := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(name, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	servers := LoadServerList(file)
	expected := ServerList{
		"main": {"host0"},
		"web":  {"web1", "web2"},
		"db":   {"db1", "db2"},
	}
	if !reflect.DeepEqual(servers, expected) {
		t.Errorf("LoadServerList() = %q, expected %q", servers, expected)
	}
}

func TestHosts(t *testing.T) {
	servers := ServerList{
		"main": {"host0", "web1"},
		"web":  {"web1", "web2"},
	}
	if hosts := servers.Hosts(""); !reflect.DeepEqual(hosts, []string{"host0", "web1", "web2"}) {
		t.Errorf("Hosts(\"\") = %q", hosts)
	}
	if hosts := servers.Hosts("web"); !reflect.DeepEqual(hosts, []string{"web1", "web2"}) {
		t.Errorf("Hosts(\"web\") = %q", hosts)
	}
	if hosts := servers.Hosts("none"); hosts != nil {
		t.Errorf("Hosts(\"none\") = %q", hosts)
	}
}
