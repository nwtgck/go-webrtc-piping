package piping_webrtc_signaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

type Offer struct {
	pipingServerUrl   string
	httpHeaders       [][]string
	peerConnection    *webrtc.PeerConnection
	offerSideId       string
	answerSideId      string
	logger            *log.Logger
	httpClient        *http.Client
	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate
}

func NewOffer(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, peerConnection *webrtc.PeerConnection, offerSideId string, answerSideId string) *Offer {
	return &Offer{
		pipingServerUrl:   pipingServerUrl,
		httpHeaders:       httpHeaders,
		peerConnection:    peerConnection,
		offerSideId:       offerSideId,
		answerSideId:      answerSideId,
		logger:            logger,
		httpClient:        httpClient,
		candidatesMux:     sync.Mutex{},
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
	}
}

func (o *Offer) Start() error {
	errCh := make(chan error)

	o.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		o.logger.Printf("OnICECandidate: %s", c)
		if c == nil {
			return
		}

		o.candidatesMux.Lock()
		defer o.candidatesMux.Unlock()

		desc := o.peerConnection.RemoteDescription()
		if desc == nil {
			o.pendingCandidates = append(o.pendingCandidates, c)
		} else if err := o.sendCandidate(c); err != nil {
			errCh <- err
		}
	})

	offer, err := o.peerConnection.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err := o.peerConnection.SetLocalDescription(offer); err != nil {
		return err
	}

	initial := InitialJson{Version: 1}
	initialBytes, err := json.Marshal(initial)
	if err != nil {
		return err
	}
	for {
		res, err := o.httpClient.Post(fmt.Sprintf("%s/%s-%s", o.pipingServerUrl, o.offerSideId, o.answerSideId), "application/json; charset=utf-8", bytes.NewReader(initialBytes))
		if err != nil {
			goto retry
		}
		if res.StatusCode != 200 {
			err = fmt.Errorf("initial status=%d", res.StatusCode)
			goto retry
		}
		if _, err = io.Copy(io.Discard, res.Body); err != nil {
			goto retry
		}
		if err = res.Body.Close(); err != nil {
			goto retry
		}
		break
	retry:
		time.Sleep(3 * time.Second)
		continue
	}

	go func() {
		// TODO: finish loop when connected
		for {
			candidate, err := receiveCandidate(o.httpClient, o.pipingServerUrl, o.httpHeaders, o.offerSideId, o.answerSideId)
			if err != nil {
				errCh <- err
				return
			}
			if err := o.peerConnection.AddICECandidate(*candidate); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		var sdp *webrtc.SessionDescription
		var err error
		for {
			sdp, err = receiveSdp(o.logger, o.httpClient, o.pipingServerUrl, o.httpHeaders, o.offerSideId, o.answerSideId)
			if err != nil {
				o.logger.Printf("failed to receive sdp: %+v", err)
				time.Sleep(3 * time.Second)
				continue
			}
			break
		}
		o.logger.Printf("sdp received")
		if err := o.peerConnection.SetRemoteDescription(*sdp); err != nil {
			errCh <- err
			return
		}
		o.candidatesMux.Lock()
		defer o.candidatesMux.Unlock()
		for _, c := range o.pendingCandidates {
			for {
				if err := o.sendCandidate(c); err != nil {
					o.logger.Printf("failed to send candidate")
					time.Sleep(3 * time.Second)
					continue
				}
				break
			}
		}
	}()

	for {
		if err := sendSdp(o.logger, o.httpClient, o.pipingServerUrl, o.httpHeaders, o.offerSideId, o.answerSideId, &offer); err != nil {
			o.logger.Printf("error: %+v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}

	return <-errCh
}

func (o *Offer) sendCandidate(candidate *webrtc.ICECandidate) error {
	return sendCandidate(o.logger, o.httpClient, o.pipingServerUrl, o.httpHeaders, o.offerSideId, o.answerSideId, candidate)
}
