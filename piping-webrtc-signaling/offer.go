package piping_webrtc_signaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Offer struct {
	pipingServerUrl *url.URL
	httpHeaders     [][]string
	peerConnection  *webrtc.PeerConnection
	offerSideId     string
	answerSideId    string
	logger          *log.Logger
	httpClient      *http.Client
}

func NewOffer(logger *log.Logger, httpClient *http.Client, pipingServerUrlStr string, httpHeaders [][]string, peerConnection *webrtc.PeerConnection, offerSideId string, answerSideId string) (*Offer, error) {
	pipingServerUrl, err := url.Parse(pipingServerUrlStr)
	if err != nil {
		return nil, err
	}
	return &Offer{
		pipingServerUrl: pipingServerUrl,
		httpHeaders:     httpHeaders,
		peerConnection:  peerConnection,
		offerSideId:     offerSideId,
		answerSideId:    answerSideId,
		logger:          logger,
		httpClient:      httpClient,
	}, nil
}

func (o *Offer) Start() error {
	errCh := make(chan error)

	candidatesMux := sync.Mutex{}
	pendingCandidates := make([]*webrtc.ICECandidate, 0)
	candidateFinished := false
	notifiedCandidateFinish := false

	o.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		o.logger.Printf("OnICECandidate: %s", c)

		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := o.peerConnection.RemoteDescription()

		if c == nil {
			candidateFinished = true
			if desc == nil {
				return
			}
			if err := o.sendCandidates([]*webrtc.ICECandidate{}); err != nil {
				errCh <- err
				return
			}
			notifiedCandidateFinish = true
			return
		}

		if desc == nil {
			pendingCandidates = append(pendingCandidates, c)
		} else if err := o.sendCandidates([]*webrtc.ICECandidate{c}); err != nil {
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
		res, err := o.httpClient.Post(urlJoin(o.pipingServerUrl, sha256String(fmt.Sprintf("%s-%s", o.offerSideId, o.answerSideId))), "application/json; charset=utf-8", bytes.NewReader(initialBytes))
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
		for {
			candidates, err := receiveCandidates(o.httpClient, o.pipingServerUrl, o.httpHeaders, o.offerSideId, o.answerSideId)
			if err != nil {
				errCh <- err
				return
			}
			if len(candidates) == 0 {
				break
			}
			for _, candidate := range candidates {
				if err := o.peerConnection.AddICECandidate(candidate); err != nil {
					errCh <- err
					return
				}
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
		candidatesMux.Lock()
		defer candidatesMux.Unlock()
		if len(pendingCandidates) != 0 {
			for {
				if err := o.sendCandidates(pendingCandidates); err != nil {
					o.logger.Printf("failed to send candidates")
					time.Sleep(3 * time.Second)
					continue
				}
				break
			}
		}
		if candidateFinished && !notifiedCandidateFinish {
			if err := o.sendCandidates([]*webrtc.ICECandidate{}); err != nil {
				errCh <- err
			}
			notifiedCandidateFinish = true
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

func (o *Offer) sendCandidates(candidates []*webrtc.ICECandidate) error {
	return sendCandidates(o.logger, o.httpClient, o.pipingServerUrl, o.httpHeaders, o.offerSideId, o.answerSideId, candidates)
}
