package main

import (
	"fmt"
	"io"
)

// cmdSweep will sweep the coin to USD rate over a grid (not yet implemented).
func cmdSweep(_ []string, _ io.Writer, _ io.Writer) error {
	return fmt.Errorf("sweep: not implemented")
}
