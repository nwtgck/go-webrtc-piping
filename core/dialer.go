// base: https://github.com/pion/webrtc/tree/80e5cdda5687d696556f2f2605a4c83f61ac3a08/examples/pion-to-pion

package core

import (
	"fmt"
	piping_webrtc_signaling "github.com/nwtgck/go-webrtc-piping-tunnel/piping-webrtc-signaling"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
)

func Dialer(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, networkType NetworkType, port uint16, path string) error {
	logger.Printf("answer-side")
	errCh := make(chan error)

	var peerConnection *webrtc.PeerConnection
	var err error
	if networkType == NetworkTypeTcp {
		peerConnection, err = NewDetachablePeerConnection(createConfig())
	} else {
		// NOTE: UDP does not need to detach
		peerConnection, err = webrtc.NewPeerConnection(createConfig())
	}
	if err != nil {
		return err
	}
	defer func() {
		if err := peerConnection.Close(); err != nil {
			logger.Printf("cannot close peerConnection: %v\n", err)
		}
	}()

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		logger.Printf("Peer Connection State has changed: %s\n", s.String())

		switch s {
		case webrtc.PeerConnectionStateFailed:
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			logger.Printf("Peer Connection has gone to failed exiting")
			errCh <- fmt.Errorf("PeerConnectionStateFailed")
		case webrtc.PeerConnectionStateDisconnected:
			errCh <- nil
		}
	})
	switch networkType {
	case NetworkTypeTcp:
		tcpDialer(logger, peerConnection, port)
	case NetworkTypeUdp:
		udpDialer(logger, peerConnection, port)
	}

	go func() {
		answer, err := piping_webrtc_signaling.NewAnswer(logger, httpClient, pipingServerUrl, httpHeaders, peerConnection, answerSideId(path), offerSideId(path))
		if err != nil {
			errCh <- err
			return
		}
		if err := answer.Start(); err != nil {
			errCh <- err
		}
	}()

	return <-errCh
}

func tcpDialer(logger *log.Logger, peerConnection *webrtc.PeerConnection, port uint16) {
	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		logger.Printf("OnDataChannel")
		// Register channel opening handling
		d.OnOpen(func() {
			logger.Printf("OnOpen")
			raw, err := d.Detach()
			if err != nil {
				logger.Printf("failed to detach: %+v", err)
				return
			}
			conn, err := net.Dial("tcp", ":"+strconv.Itoa(int(port)))
			if err != nil {
				logger.Printf("failed to dial", err)
				raw.Close()
				return
			}
			go io.Copy(raw, conn)
			go io.Copy(conn, raw)
		})
	})
}

func udpDialer(logger *log.Logger, peerConnection *webrtc.PeerConnection, port uint16) {
	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		logger.Printf("OnDataChannel")
		// TODO: hard code: 127.0.0.1
		conn, err := net.Dial("udp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))))
		if err != nil {
			log.Printf("failed to dial: %v", err)
			d.Close()
			return
		}
		// Register channel opening handling
		d.OnOpen(func() {
			logger.Printf("data channel OnOpen")
			var buf [65536]byte
			for {
				n, err := conn.Read(buf[:])
				if err != nil {
					logger.Printf("failed to read: %+v", err)
					return
				}
				if err := d.Send(buf[:n]); err != nil {
					logger.Printf("failed to send: %+v", err)
					return
				}
			}
		})

		d.OnMessage(func(msg webrtc.DataChannelMessage) {
			if _, err := conn.Write(msg.Data); err != nil {
				logger.Printf("failed to write: %+v", err)
			}
		})
	})
}
