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

func Listener(logger *log.Logger, pipingServerUrl string, tcpPort uint16, path string) error {
	logger.Printf("listener: offer-side")
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

	// Create a datachannel with label 'data'
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		return err
	}
	// NOTE: Close immediately because main purpose of creating the data channel for avoiding empty SDP
	// TODO: find better way
	if err := dataChannel.Close(); err != nil {
		return err
	}

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		logger.Printf("Peer Connection State has changed: %s\n", s.String())

		switch s {
		case webrtc.PeerConnectionStateFailed:
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure. It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			logger.Println("Peer Connection has gone to failed exiting")
			errCh <- fmt.Errorf("PeerConnectionStateFailed")
		case webrtc.PeerConnectionStateDisconnected:
			errCh <- nil
		}
	})

	go func() {
		ln, err := net.Listen("tcp", ":"+strconv.Itoa(int(tcpPort)))
		if err != nil {
			errCh <- err
			return
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				errCh <- err
				return
			}
			logger.Printf("accepted")
			dataChannel, err := peerConnection.CreateDataChannel("data", nil)
			if err != nil {
				errCh <- err
				return
			}
			dataChannel.OnOpen(func() {
				logger.Printf("OnOpen in listener")
				raw, err := dataChannel.Detach()
				if err != nil {
					logger.Printf("failed to detach: %+v", err)
					return
				}
				go io.Copy(raw, conn)
				go io.Copy(conn, raw)
			})
		}
	}()

	go func() {
		offer := piping_webrtc_signaling.NewOffer(logger, pipingServerUrl, peerConnection, offerSideId(path), answerSideId(path))
		if err := offer.Start(); err != nil {
			errCh <- err
		}
	}()

	return <-errCh
}
