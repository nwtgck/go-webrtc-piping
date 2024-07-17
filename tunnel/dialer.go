// base: https://github.com/pion/webrtc/tree/80e5cdda5687d696556f2f2605a4c83f61ac3a08/examples/pion-to-pion

package tunnel

import (
	"fmt"
	piping_webrtc_signaling "github.com/nwtgck/go-webrtc-piping/piping-webrtc-signaling"
	"github.com/pion/webrtc/v3"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
)

func Dialer(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, networkType NetworkType, port uint16, path string, webrtcConfig webrtc.Configuration) error {
	logger.Printf("answer-side")
	var eg errgroup.Group

	var peerConnection *webrtc.PeerConnection
	var err error
	if networkType == NetworkTypeTcp {
		peerConnection, err = NewDetachablePeerConnection(webrtcConfig)
	} else {
		// NOTE: UDP does not need to detach
		peerConnection, err = webrtc.NewPeerConnection(webrtcConfig)
	}
	if err != nil {
		return err
	}
	defer func() {
		if err := peerConnection.Close(); err != nil {
			logger.Printf("cannot close peerConnection: %v\n", err)
		}
	}()

	eg.Go(func() error {
		errCh := make(chan error)
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
			default:
			}
		})
		return <-errCh
	})

	switch networkType {
	case NetworkTypeTcp:
		tcpDialer(logger, peerConnection, port)
	case NetworkTypeUdp:
		udpDialer(logger, peerConnection, port)
	}

	eg.Go(func() error {
		answer, err := piping_webrtc_signaling.NewAnswer(logger, httpClient, pipingServerUrl, httpHeaders, peerConnection, answerSideId(path), offerSideId(path))
		if err != nil {
			return err
		}
		if err := answer.Start(); err != nil {
			return err
		}
		return nil
	})

	return eg.Wait()
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
				logger.Printf("failed to dial: %+v", err)
				if err := raw.Close(); err != nil {
					logger.Printf("failed to close raw: %+v", err)
				}
				return
			}
			go func() {
				if _, err := io.Copy(raw, conn); err != nil {
					logger.Printf("failed to copy conn to raw: %+v", err)
				}
			}()
			go func() {
				if _, err := io.Copy(conn, raw); err != nil {
					logger.Printf("failed to copy raw to conn: %+v", err)
				}
			}()
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
			log.Printf("failed to dial: %+v", err)
			if err := d.Close(); err != nil {
				log.Printf("failed to close: %+v", err)
			}
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
