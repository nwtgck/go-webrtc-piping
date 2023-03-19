package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/nwtgck/go-webrtc-piping/core"
	"github.com/nwtgck/go-webrtc-piping/version"
	"github.com/spf13/cobra"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ServerUrlEnvName = "PIPING_SERVER"
)

var flags struct {
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
	cobra.OnInitialize()
	defaultServer, ok := os.LookupEnv(ServerUrlEnvName)
	if !ok {
		defaultServer = "https://ppng.io"
	}
	RootCmd.Flags().StringVarP(&flags.pipingServerUrl, "server", "s", defaultServer, "Piping Server URL")
	RootCmd.PersistentFlags().StringVar(&flags.dnsServer, "dns-server", "", "DNS server (e.g. 1.1.1.1:53)")
	// --insecure, -k is inspired by curl
	RootCmd.PersistentFlags().BoolVarP(&flags.insecure, "insecure", "k", false, "Allow insecure server connections when using SSL")
	RootCmd.PersistentFlags().StringArrayVarP(&flags.httpHeaderKeyValueStrs, "header", "H", []string{}, "HTTP header")
	RootCmd.Flags().BoolVarP(&flags.showsVersion, "version", "V", false, "show version")
	RootCmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "verbose output")
	RootCmd.Flags().BoolVarP(&flags.listens, "listen", "l", false, "listen mode")
	RootCmd.Flags().BoolVarP(&flags.usesUdp, "udp", "u", false, "UDP")
}

var RootCmd = &cobra.Command{
	Use:   os.Args[0],
	Short: "webrtc-piping",
	Long:  "WebRTC tunnel with Piping Server WebRTC signaling",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flags.showsVersion {
			fmt.Println(version.Version)
			return nil
		}
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

		if flags.usesUdp {
			if flags.listens {
				return core.Listener(logger, httpClient, flags.pipingServerUrl, httpHeaders, core.NetworkTypeUdp, uint16(port), path)
			}
			return core.Dialer(logger, httpClient, flags.pipingServerUrl, httpHeaders, core.NetworkTypeUdp, uint16(port), path)
		}
		if flags.listens {
			return core.Listener(logger, httpClient, flags.pipingServerUrl, httpHeaders, core.NetworkTypeTcp, uint16(port), path)
		}
		return core.Dialer(logger, httpClient, flags.pipingServerUrl, httpHeaders, core.NetworkTypeTcp, uint16(port), path)
	},
}

func createHttpClient(insecureSkipVerify bool) *http.Client {
	// Set insecure or not
	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		ForceAttemptHTTP2: true,
	}
	return &http.Client{Transport: tr}
}

// Set default resolver for HTTP client
func createDialContext(dnsServer string) func(ctx context.Context, network, address string) (net.Conn, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", dnsServer)
		},
	}

	// Resolver for HTTP
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{
			Timeout:  time.Millisecond * time.Duration(10000),
			Resolver: resolver,
		}
		return d.DialContext(ctx, network, address)
	}
}

func parseHeaderKeyValueStrs(strKeyValues []string) ([][]string, error) {
	var keyValues [][]string
	for _, str := range strKeyValues {
		splitted := strings.SplitN(str, ":", 2)
		if len(splitted) != 2 {
			return nil, fmt.Errorf("invalid header format '%s'", str)
		}
		keyValues = append(keyValues, splitted)
	}
	return keyValues, nil
}
