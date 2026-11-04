package appmanager

import (
	"net"
	"os"
	"testing"
)

func TestRuntimeAddressValidation(t *testing.T) {
	for _, value := range []string{"", "0.0.0.0:11434", "localhost:11434", "127.0.0.1:59420", "127.0.0.1:59421", "127.0.0.1:65536", "127.0.0.1:80", "127.0.0.1:11435\nOTHER=secret"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("OLLAMA_HOST", value)
			if _, _, err := RuntimeAddress(); err == nil {
				t.Fatal("accepted invalid runtime address")
			}
		})
	}
	t.Setenv("OLLAMA_HOST", "127.0.0.1:11435")
	endpoint, err := RuntimeEndpoint()
	if err != nil || endpoint != "http://127.0.0.1:11435" {
		t.Fatalf("endpoint %q: %v", endpoint, err)
	}
}
func TestExactProcessSocket(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if !exactProcessSocket(unitIdentity{pid: os.Getpid()}, port) {
		t.Fatal("own listener not attributed")
	}
	if exactProcessSocket(unitIdentity{pid: 99999999}, port) {
		t.Fatal("absent process attributed")
	}
}
func TestParseListenerRejectsWildcard(t *testing.T) {
	raw := []byte("header\n 0: 00000000:2CAA 00000000:0000 0A 0:0 0:0 0 1000 0 123\n")
	if _, ok := parseListeners(raw, 11434, false); ok {
		t.Fatal("wildcard accepted")
	}
}
