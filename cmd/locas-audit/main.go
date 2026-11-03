// locas-audit verifies an existing service journal without modifying it.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/p8reit/LocasOS/internal/auditlog"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || args[0] != "verify" {
		fmt.Fprintln(stderr, "usage: locas-audit verify DIRECTORY")
		return 2
	}
	report, err := auditlog.Verify(args[1])
	if err != nil {
		fmt.Fprintln(stderr, "audit verification failed:", err)
		return 1
	}
	if err = json.NewEncoder(stdout).Encode(report); err != nil {
		return 1
	}
	return 0
}
