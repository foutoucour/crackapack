package main

import (
	"fmt"
	"io"
)

// cmdCompare will compare two simulation results (not yet implemented).
func cmdCompare(_ []string, _ io.Writer, _ io.Writer) error {
	return fmt.Errorf("compare: not implemented")
}
