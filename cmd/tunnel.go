package cmd

import (
	"fmt"
	"github.com/nwtgck/go-webrtc-piping/tunnel"
	"github.com/spf13/cobra"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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
	Use:   "tunnel",
	Short: "Tunneling TCP or UDP",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("port and path are required")
		}
		portStr := args[0]
		path := args[1]
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return err
		}

		var logger *log.Logger
		if flags.verbose {
			logger = log.New(os.Stderr, "", log.LstdFlags)
		} else {
			logger = log.New(io.Discard, "", 0)
		}

		httpClient := createHttpClient(flags.insecure)
		if flags.dnsServer != "" {
			httpClient.Transport.(*http.Transport).DialContext = createDialContext(flags.dnsServer)
		}

		httpHeaders, err := parseHeaderKeyValueStrs(flags.httpHeaderKeyValueStrs)
		if err != nil {
			return err
		}

		if tunnelFlags.usesUdp {
			if tunnelFlags.listens {
				return tunnel.Listener(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeUdp, uint16(port), path)
			}
			return tunnel.Dialer(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeUdp, uint16(port), path)
		}
		if tunnelFlags.listens {
			return tunnel.Listener(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeTcp, uint16(port), path)
		}
		return tunnel.Dialer(logger, httpClient, flags.pipingServerUrl, httpHeaders, tunnel.NetworkTypeTcp, uint16(port), path)
	},
}