package piping_webrtc_signaling

import (
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Answer struct {
	pipingServerUrl *url.URL
	httpHeaders     [][]string
	peerConnection  *webrtc.PeerConnection
	answerSideId    string
	offerSideId     string
	logger          *log.Logger
	httpClient      *http.Client
}

func NewAnswer(logger *log.Logger, httpClient *http.Client, pipingServerUrlStr string, httpHeaders [][]string, peerConnection *webrtc.PeerConnection, answerSideId string, offerSideId string) (*Answer, error) {
	pipingServerUrl, err := url.Parse(pipingServerUrlStr)
	if err != nil {
		return nil, err
	}
	return &Answer{
		pipingServerUrl: pipingServerUrl,
		httpHeaders:     httpHeaders,
		peerConnection:  peerConnection,
		answerSideId:    answerSideId,
		offerSideId:     offerSideId,
		logger:          logger,
		httpClient:      httpClient,
	}, nil
}

func (a *Answer) Start() error {
	errCh := make(chan error)
	var wg sync.WaitGroup

	candidatesMux := sync.Mutex{}
	pendingCandidates := make([]*webrtc.ICECandidate, 0)
	candidateFinished := false
	notifiedCandidateFinish := false

	a.peerConnection.OnICECandidate(func(c *webrtc.ICECandidate) {
		candidatesMux.Lock()
		defer candidatesMux.Unlock()

		desc := a.peerConnection.RemoteDescription()

		if c == nil {
			candidateFinished = true
			if desc == nil {
				return
			}
			if err := a.sendCandidates([]*webrtc.ICECandidate{}); err != nil {
				errCh <- err
				return
			}
			notifiedCandidateFinish = true
			return
		}

		if desc == nil {
			pendingCandidates = append(pendingCandidates, c)
		} else if err := a.sendCandidates([]*webrtc.ICECandidate{c}); err != nil {
			errCh <- err
		}
	})

	var offerInitial OfferInitialJson
	for {
		err := func() error {
			offerInitialBytes, err := httpGetWithHeaders(a.httpClient, urlJoin(a.pipingServerUrl, sha256String(fmt.Sprintf("%s-%s", a.offerSideId, a.answerSideId))), a.httpHeaders)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(offerInitialBytes, &offerInitial); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			a.logger.Printf("error: %+v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	a.logger.Printf("offerInitial: %+v", offerInitial)

	answerInitial := AnswerInitialJson{Version: 1}
	answerInitialBytes, err := json.Marshal(answerInitial)
	if err != nil {
		return err
	}
	for {
		err := pipingPostJson(a.httpClient, urlJoin(a.pipingServerUrl, sha256String(fmt.Sprintf("%s-%s", a.answerSideId, a.offerSideId))), a.httpHeaders, answerInitialBytes)
		if err != nil {
			a.logger.Printf("failed to send answerInitial: %+v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			candidates, err := receiveCandidates(a.httpClient, a.pipingServerUrl, a.httpHeaders, a.answerSideId, a.offerSideId)
			if err != nil {
				errCh <- err
				return
			}
			if len(candidates) == 0 {
				break
			}
			a.logger.Printf("candidate received")
			for _, candidate := range candidates {
				if err := a.peerConnection.AddICECandidate(candidate); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
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
		candidatesMux.Lock()
		defer candidatesMux.Unlock()
		if len(pendingCandidates) != 0 {
			for {
				if err = a.sendCandidates(pendingCandidates); err != nil {
					a.logger.Printf("failed to send candidates: %+v", err)
					time.Sleep(3 * time.Second)
					continue
				}
				break
			}
		}
		if candidateFinished && !notifiedCandidateFinish {
			if err := a.sendCandidates([]*webrtc.ICECandidate{}); err != nil {
				errCh <- err
			}
			notifiedCandidateFinish = true
		}
	}()

	wg.Wait()
	return <-errCh
}

func (a *Answer) sendCandidates(candidates []*webrtc.ICECandidate) error {
	return sendCandidates(a.logger, a.httpClient, a.pipingServerUrl, a.httpHeaders, a.answerSideId, a.offerSideId, candidates)
}
