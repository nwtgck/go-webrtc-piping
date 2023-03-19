package cmd

import (
	"fmt"
	"github.com/nwtgck/go-webrtc-piping/duplex"
	"github.com/spf13/cobra"
	"io"
	"log"
	"net/http"
	"os"
)

var duplexFlags struct{}

func init() {
	RootCmd.AddCommand(DuplexCmd)
}

var DuplexCmd = &cobra.Command{
	Use:   "duplex",
	Short: "Duplex communication",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("local id and remote id are required")
		}
		localId := args[0]
		remoteId := args[1]
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
		webrtcConfig := createWebrtcConfig()
		if localId < remoteId {
			return duplex.HandleOffer(logger, httpClient, flags.pipingServerUrl, httpHeaders, localId, remoteId, webrtcConfig)
		} else {
			return duplex.HandleAnswer(logger, httpClient, flags.pipingServerUrl, httpHeaders, localId, remoteId, webrtcConfig)
		}
	},
}
