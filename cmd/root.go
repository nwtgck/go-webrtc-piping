package cmd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/nwtgck/go-webrtc-piping/version"
	"github.com/pion/webrtc/v3"
	"github.com/spf13/cobra"
	"net"
	"net/http"
	"os"
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
	iceServers             []iceServerFlag
	showsVersion           bool
	verbose                bool
}

type iceServerFlag struct {
	URLs       iceServerFlagUrls `json:"urls"`
	Username   string            `json:"username,omitempty"`
	Credential string            `json:"credential,omitempty"`
	// NOTE: credentialType is deprecated
}

// iceServerFlagUrls is string or []string
type iceServerFlagUrls struct {
	values []string
}

var _ json.Marshaler = (*iceServerFlagUrls)(nil)
var _ json.Unmarshaler = (*iceServerFlagUrls)(nil)

func (u *iceServerFlagUrls) MarshalJSON() ([]byte, error) {
	if len(u.values) == 1 {
		return json.Marshal(u.values[0])
	}
	return json.Marshal(u.values)
}

func (u *iceServerFlagUrls) UnmarshalJSON(b []byte) error {
	var stringValue string
	err := json.Unmarshal(b, &stringValue)
	if err == nil {
		u.values = []string{stringValue}
		return nil
	}
	err = json.Unmarshal(b, &u.values)
	if err == nil {
		return nil
	}
	return fmt.Errorf("urls is not string or string array")
}

func init() {
	cobra.OnInitialize()
	defaultServer, ok := os.LookupEnv(ServerUrlEnvName)
	if !ok {
		defaultServer = "https://ppng.io"
	}
	flags.iceServers = []iceServerFlag{
		{URLs: iceServerFlagUrls{values: []string{"stun:stun.l.google.com:19302"}}},
	}
	RootCmd.PersistentFlags().StringVarP(&flags.pipingServerUrl, "server", "s", defaultServer, "Piping Server URL")
	RootCmd.PersistentFlags().StringVar(&flags.dnsServer, "dns-server", "", "DNS server (e.g. 1.1.1.1:53)")
	// --insecure, -k is inspired by curl
	RootCmd.PersistentFlags().BoolVarP(&flags.insecure, "insecure", "k", false, "Allow insecure server connections when using SSL")
	RootCmd.PersistentFlags().StringArrayVarP(&flags.httpHeaderKeyValueStrs, "header", "H", []string{}, "HTTP header")
	RootCmd.PersistentFlags().VarP(&JsonFlag{Value: &flags.iceServers}, "ice-servers", "", "ICE servers")
	RootCmd.PersistentFlags().BoolVarP(&flags.showsVersion, "version", "V", false, "show version")
	RootCmd.PersistentFlags().BoolVarP(&flags.verbose, "verbose", "v", false, "verbose output")
}

var RootCmd = &cobra.Command{
	Use:          os.Args[0],
	Short:        "webrtc-piping",
	Long:         "WebRTC tunnel with Piping Server WebRTC signaling",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if flags.showsVersion {
			fmt.Println(version.Version)
			return nil
		}
		return cmd.Help()
	},
}

func createHttpClient(insecureSkipVerify bool, dnsServer string /* empty string OK */) *http.Client {
	// Set insecure or not
	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: insecureSkipVerify},
		ForceAttemptHTTP2: true,
	}
	if dnsServer != "" {
		tr.DialContext = createDialContext(dnsServer)
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

func createWebrtcConfig() webrtc.Configuration {
	iceServer := make([]webrtc.ICEServer, len(flags.iceServers))
	for i, d := range flags.iceServers {
		iceServer[i] = webrtc.ICEServer{
			URLs:       d.URLs.values,
			Username:   d.Username,
			Credential: d.Credential,
		}
	}
	return webrtc.Configuration{
		ICEServers: iceServer,
	}
}
