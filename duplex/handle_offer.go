// base: https://github.com/pion/webrtc/tree/80e5cdda5687d696556f2f2605a4c83f61ac3a08/examples/pion-to-pion

package duplex

import (
	"fmt"
	piping_webrtc_signaling "github.com/nwtgck/go-webrtc-piping/piping-webrtc-signaling"
	"github.com/pion/webrtc/v3"
	"log"
	"net/http"
)

func HandleOffer(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, localId string, remoteId string, webrtcConfig webrtc.Configuration) error {
	logger.Printf("offer-side")
	errCh := make(chan error)

	// Create a new RTCPeerConnection
	peerConnection, err := webrtc.NewPeerConnection(webrtcConfig)
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

	// Register channel opening handling
	dataChannelCh := make(chan *webrtc.DataChannel)
	dataChannel.OnOpen(func() {
		logger.Printf("OnOpen")
		dataChannelCh <- dataChannel
	})
	dcToStdoutErrCh := registerOnMessageForDataChannelToStdout(logger, dataChannel)

	stdinToDcErrCh := make(chan error)
	go func() {
		dataChannel := <-dataChannelCh
		stdinToDcErrCh <- stdinToDataChannel(logger, dataChannel)
	}()

	go func() {
		if err := <-dcToStdoutErrCh; err != nil {
			errCh <- err
		}
		if err := <-stdinToDcErrCh; err != nil {
			errCh <- err
		}
		errCh <- nil
	}()

	go func() {
		offer, err := piping_webrtc_signaling.NewOffer(logger, httpClient, pipingServerUrl, httpHeaders, peerConnection, localId, remoteId)
		if err != nil {
			errCh <- err
			return
		}
		if err := offer.Start(); err != nil {
			errCh <- err
		}
		logger.Printf("offer finished")
	}()

	return <-errCh
}
