// base: https://github.com/pion/webrtc/tree/80e5cdda5687d696556f2f2605a4c83f61ac3a08/examples/pion-to-pion

package core

import (
	"fmt"
	piping_webrtc_signaling "github.com/nwtgck/go-webrtc-piping-tunnel/piping-webrtc-signaling"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net"
	"strconv"
)

func Dialer(logger *log.Logger, pipingServerUrl string, tcpPort uint16, path string) error {
	logger.Printf("answer-side")
	errCh := make(chan error)

	// Create a new RTCPeerConnection
	peerConnection, err := NewDetachablePeerConnection(createConfig())
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

	// Register data channel creation handling
	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		logger.Printf("OnDataChannel")
		// Register channel opening handling
		tcpDialer(logger, d, tcpPort)
	})

	go func() {
		answer := piping_webrtc_signaling.NewAnswer(logger, pipingServerUrl, peerConnection, answerSideId(path), offerSideId(path))
		if err := answer.Start(); err != nil {
			errCh <- err
		}
	}()

	return <-errCh
}

func tcpDialer(logger *log.Logger, dataChannel *webrtc.DataChannel, tcpPort uint16) {
	dataChannel.OnOpen(func() {
		logger.Printf("OnOpen")
		raw, err := dataChannel.Detach()
		if err != nil {
			logger.Printf("failed to detach: %+v", err)
			return
		}
		conn, err := net.Dial("tcp", ":"+strconv.Itoa(int(tcpPort)))
		if err != nil {
			logger.Printf("failed to dial", err)
			raw.Close()
			return
		}
		go io.Copy(raw, conn)
		go io.Copy(conn, raw)
	})
}
