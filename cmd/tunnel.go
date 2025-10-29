package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/nwtgck/go-webrtc-piping/tunnel"
	"github.com/spf13/cobra"
)

var tunnelFlags struct {
	pipingServerUrl        string
	dnsServer              string
	insecure               bool
	httpHeaderKeyValueStrs []string
	showsVersion           bool
	verbose                bool
	listens                bool
	usesUdp                bool
}

func init() {
	RootCmd.AddCommand(TunnelCmd)
	TunnelCmd.Flags().BoolVarP(&tunnelFlags.listens, "listen", "l", false, "listen mode")
	TunnelCmd.Flags().BoolVarP(&tunnelFlags.usesUdp, "udp", "u", false, "UDP")
}

var TunnelCmd = &cobra.Command{
	Use:   "tunnel <addr> <path>",
	Short: "Tunneling TCP or UDP",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("addr and path are required")
		}
		addrStr := args[0]
		path := args[1]

		var logger *log.Logger
		if flags.verbose {
			logger = log.New(os.Stderr, "", log.LstdFlags)
		} else {
			logger = log.New(io.Discard, "", 0)
		}

		httpClient := createHttpClient(flags.insecure, flags.dnsServer)
		httpHeaders, err := parseHeaderKeyValueStrs(flags.httpHeaderKeyValueStrs)
		if err != nil {
			return err
		}

		webrtcConfig := createWebrtcConfig()
		if tunnelFlags.usesUdp {
			if tunnelFlags.listens {
				return tunnel.Listener(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeUdp, addrStr, path, webrtcConfig)
			}
			return tunnel.Dialer(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeUdp, addrStr, path, webrtcConfig)
		}
		if tunnelFlags.listens {
			return tunnel.Listener(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeTcp, addrStr, path, webrtcConfig)
		}
		return tunnel.Dialer(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeTcp, addrStr, path, webrtcConfig)
	},
}
