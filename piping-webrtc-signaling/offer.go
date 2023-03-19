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
	var wg sync.WaitGroup

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

	offerInitial := OfferInitialJson{Version: 1}
	offerInitialBytes, err := json.Marshal(offerInitial)
	if err != nil {
		return err
	}
	for {
		err := pipingPostJson(o.httpClient, urlJoin(o.pipingServerUrl, sha256String(fmt.Sprintf("%s-%s", o.offerSideId, o.answerSideId))), o.httpHeaders, offerInitialBytes)
		if err != nil {
			o.logger.Printf("failed to send offerInitial: %+v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	var answerInitial AnswerInitialJson
	for {
		err := func() error {
			answerInitialBytes, err := httpGetWithHeaders(o.httpClient, urlJoin(o.pipingServerUrl, sha256String(fmt.Sprintf("%s-%s", o.answerSideId, o.offerSideId))), o.httpHeaders)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(answerInitialBytes, &answerInitial); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			o.logger.Printf("error: %+v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	o.logger.Printf("answerInitial: %+v", answerInitial)
	if answerInitial.Version > 1 {
		return fmt.Errorf("unsupported answer-side version: %d", answerInitial.Version)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
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

	wg.Add(1)
	go func() {
		defer wg.Done()
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

	wg.Wait()
	return <-errCh
}

func (o *Offer) sendCandidates(candidates []*webrtc.ICECandidate) error {
	return sendCandidates(o.logger, o.httpClient, o.pipingServerUrl, o.httpHeaders, o.offerSideId, o.answerSideId, candidates)
}
