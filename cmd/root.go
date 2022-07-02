package cmd

import (
	"fmt"
	"github.com/nwtgck/go-webrtc-piping-tunnel/core"
	"github.com/nwtgck/go-webrtc-piping-tunnel/version"
	"github.com/spf13/cobra"
	"io"
	"log"
	"os"
	"strconv"
)

const (
	ServerUrlEnvName = "PIPING_SERVER"
)

var flags struct {
	pipingServerUrl string
	showsVersion    bool
	verbose         bool
	listens         bool
}

func init() {
	cobra.OnInitialize()
	defaultServer, ok := os.LookupEnv(ServerUrlEnvName)
	if !ok {
		defaultServer = "https://ppng.io"
	}
	RootCmd.Flags().StringVarP(&flags.pipingServerUrl, "server", "s", defaultServer, "Piping Server URL")
	RootCmd.Flags().BoolVarP(&flags.showsVersion, "version", "V", false, "show version")
	RootCmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "verbose output")
	RootCmd.Flags().BoolVarP(&flags.listens, "listen", "l", false, "listen mode")
}

var RootCmd = &cobra.Command{
	Use:   os.Args[0],
	Short: "webrtc-piping-tunnel",
	Long:  "WebRTC tunnel with Piping Server WebRTC signaling",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flags.showsVersion {
			fmt.Println(version.Version)
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("port and path are required")
		}
		// TODO: support UDP
		tcpPortStr := args[0]
		path := args[1]
		tcpPort, err := strconv.Atoi(tcpPortStr)
		if err != nil {
			return err
		}
		var logger *log.Logger
		if flags.verbose {
			logger = log.New(os.Stderr, "", log.LstdFlags)
		} else {
			logger = log.New(io.Discard, "", 0)
		}
		if flags.listens {
			return core.Listener(logger, flags.pipingServerUrl, uint16(tcpPort), path)
		}
		return core.Dialer(logger, flags.pipingServerUrl, uint16(tcpPort), path)
	},
}
