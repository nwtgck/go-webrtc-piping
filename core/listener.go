// base: https://github.com/pion/webrtc/tree/80e5cdda5687d696556f2f2605a4c83f61ac3a08/examples/pion-to-pion

package core

import (
	"fmt"
	piping_webrtc_signaling "github.com/nwtgck/go-webrtc-piping/piping-webrtc-signaling"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
)

func Listener(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, networkType NetworkType, port uint16, path string) error {
	logger.Printf("listener: offer-side")
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
		switch networkType {
		case NetworkTypeTcp:
			if err := tcpListener(logger, peerConnection, port); err != nil {
				errCh <- err
				return
			}
		case NetworkTypeUdp:
			if err := udpListener(logger, peerConnection, port); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		offer, err := piping_webrtc_signaling.NewOffer(logger, httpClient, pipingServerUrl, httpHeaders, peerConnection, offerSideId(path), answerSideId(path))
		if err != nil {
			errCh <- err
			return
		}
		if err := offer.Start(); err != nil {
			errCh <- err
		}
	}()

	return <-errCh
}

func tcpListener(logger *log.Logger, peerConnection *webrtc.PeerConnection, port uint16) error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		logger.Printf("accepted")
		dataChannel, err := peerConnection.CreateDataChannel("data", nil)
		if err != nil {
			return err
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
}

type udpAddrToDataChannelMap struct {
	inner *sync.Map
}

func (m udpAddrToDataChannelMap) Load(key *net.UDPAddr) *webrtc.DataChannel {
	dataChannel, ok := m.inner.Load(key.String())
	if !ok {
		return nil
	}
	return dataChannel.(*webrtc.DataChannel)
}

func (m udpAddrToDataChannelMap) Store(key *net.UDPAddr, value *webrtc.DataChannel) {
	m.inner.Store(key.String(), value)
}

func udpListener(logger *log.Logger, peerConnection *webrtc.PeerConnection, port uint16) error {
	var ordered = false
	var maxRetransmits uint16 = 0
	dataChannelOptions := webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	}
	raddrToDataChannel := udpAddrToDataChannelMap{inner: new(sync.Map)}

	laddr, err := net.ResolveUDPAddr("udp", ":"+strconv.Itoa(int(port)))
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		return err
	}
	var buf [65536]byte
	for {
		n, raddr, err := conn.ReadFromUDP(buf[:])
		if err != nil {
			return err
		}
		dataChannel := raddrToDataChannel.Load(raddr)
		if dataChannel == nil {
			dataChannel, err = peerConnection.CreateDataChannel("data", &dataChannelOptions)
			if err != nil {
				return err
			}

			dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
				if _, err := conn.WriteToUDP(msg.Data, raddr); err != nil {
					logger.Printf("failed to write to UDP: %+v", err)
				}
			})

			openedCh := make(chan struct{})
			dataChannel.OnOpen(func() {
				raddrToDataChannel.Store(raddr, dataChannel)
				openedCh <- struct{}{}
			})
			<-openedCh

		}
		if err := dataChannel.Send(buf[:n]); err != nil {
			return err
		}
	}
}
