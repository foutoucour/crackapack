package main

import (
	"fmt"
	"io"
)

// cmdDemo will run a sweep on the demo dataset (not yet implemented).
func cmdDemo(_ []string, _ io.Writer, _ io.Writer) error {
	return fmt.Errorf("demo: not implemented")
}
