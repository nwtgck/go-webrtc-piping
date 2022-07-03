package piping_webrtc_signaling

import (
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"log"
	"net/http"
	"sync"
	"time"
)

type Answer struct {
	pipingServerUrl   string
	httpHeaders       [][]string
	peerConnection    *webrtc.PeerConnection
	answerSideId      string
	offerSideId       string
	logger            *log.Logger
	httpClient        *http.Client
	candidatesMux     sync.Mutex
	pendingCandidates []*webrtc.ICECandidate
}

func NewAnswer(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, peerConnection *webrtc.PeerConnection, answerSideId string, offerSideId string) *Answer {
	return &Answer{
		pipingServerUrl:   pipingServerUrl,
		httpHeaders:       httpHeaders,
		peerConnection:    peerConnection,
		answerSideId:      answerSideId,
		offerSideId:       offerSideId,
		logger:            logger,
		httpClient:        httpClient,
		candidatesMux:     sync.Mutex{},
		pendingCandidates: make([]*webrtc.ICECandidate, 0),
	}
}

func (a *Answer) Start() error {
	errCh := make(chan error)

	a.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		a.candidatesMux.Lock()
		defer a.candidatesMux.Unlock()

		desc := a.peerConnection.RemoteDescription()
		if desc == nil {
			a.pendingCandidates = append(a.pendingCandidates, c)
		} else if err := a.sendCandidate(c); err != nil {
			errCh <- err
		}
	})

	var initial InitialJson
	for {
		res, err := a.httpClient.Get(fmt.Sprintf("%s/%s-%s", a.pipingServerUrl, a.offerSideId, a.answerSideId))
		if err != nil {
			goto retry
		}
		if err = json.NewDecoder(res.Body).Decode(&initial); err != nil {
			goto retry
		}
		break
	retry:
		a.logger.Printf("error: %+v", err)
		time.Sleep(3 * time.Second)
	}
	a.logger.Printf("initial: %+v", initial)

	go func() {
		// TODO: finish loop when connected
		for {
			candidate, err := receiveCandidate(a.httpClient, a.pipingServerUrl, a.httpHeaders, a.answerSideId, a.offerSideId)
			if err != nil {
				errCh <- err
				return
			}
			a.logger.Printf("candidate received")
			if err := a.peerConnection.AddICECandidate(*candidate); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		var sdp *webrtc.SessionDescription
		var err error
		for {
			sdp, err = receiveSdp(a.logger, a.httpClient, a.pipingServerUrl, a.httpHeaders, a.answerSideId, a.offerSideId)
			if err != nil {
				a.logger.Printf("failed to receive sdp: %+v", err)
				time.Sleep(3 * time.Second)
				continue
			}
			break
		}
		a.logger.Printf("sdp received")
		if err := a.peerConnection.SetRemoteDescription(*sdp); err != nil {
			errCh <- err
			return
		}
		// Create an answer to send to the other process
		answer, err := a.peerConnection.CreateAnswer(nil)
		if err != nil {
			errCh <- err
			return
		}
		for {
			if err := sendSdp(a.logger, a.httpClient, a.pipingServerUrl, a.httpHeaders, a.answerSideId, a.offerSideId, &answer); err != nil {
				a.logger.Printf("failed to send sdp: %+v", err)
				time.Sleep(3 * time.Second)
				continue
			}
			break
		}
		// Sets the LocalDescription, and starts our UDP listeners
		err = a.peerConnection.SetLocalDescription(answer)
		if err != nil {
			errCh <- err
			return
		}
		a.candidatesMux.Lock()
		defer a.candidatesMux.Unlock()
		for _, c := range a.pendingCandidates {
			for {
				if err = a.sendCandidate(c); err != nil {
					a.logger.Printf("failed to send candidate: %+v", err)
					time.Sleep(3 * time.Second)
					continue
				}
				break
			}
		}
	}()

	return <-errCh
}

func (a *Answer) sendCandidate(candidate *webrtc.ICECandidate) error {
	return sendCandidate(a.logger, a.httpClient, a.pipingServerUrl, a.httpHeaders, a.answerSideId, a.offerSideId, candidate)
}
