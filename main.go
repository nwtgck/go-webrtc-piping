package main

import (
	"fmt"
	"github.com/nwtgck/go-webrtc-piping/cmd"
	"os"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(-1)
	}
}
