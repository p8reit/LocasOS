package appmanager

import (
	"errors"
	"net"
	"os"
	"strconv"
)

// RuntimeAddress is a local-only configuration boundary, never a browser input.
// An explicitly empty/malformed value fails closed; only an unset value defaults.
func RuntimeAddress() (string, int, error) {
	raw, configured := os.LookupEnv("OLLAMA_HOST")
	if !configured {
		raw = "127.0.0.1:11434"
	}
	host, port, err := net.SplitHostPort(raw)
	if err != nil || host != "127.0.0.1" || len(port) == 0 || len(port) > 5 {
		return "", 0, errors.New("invalid local runtime configuration")
	}
	for _, digit := range port {
		if digit < '0' || digit > '9' {
			return "", 0, errors.New("invalid local runtime configuration")
		}
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1024 || number > 65535 || number == 59420 || number == 59421 {
		return "", 0, errors.New("invalid local runtime configuration")
	}
	return net.JoinHostPort(host, strconv.Itoa(number)), number, nil
}
func RuntimeEndpoint() (string, error) {
	address, _, err := RuntimeAddress()
	if err != nil {
		return "", err
	}
	return "http://" + address, nil
}
