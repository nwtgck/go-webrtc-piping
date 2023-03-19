package piping_webrtc_signaling

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"
)

type OfferInitialJson struct {
	Version uint64 `json:"version"`
}

type AnswerInitialJson struct {
	Version uint64 `json:"version"`
}

func sha256String(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

// (base: https://stackoverflow.com/a/34668130/2885946)
func urlJoin(u *url.URL, p ...string) string {
	uCloned := *u
	uCloned.Path = path.Join(append([]string{uCloned.Path}, p...)...)
	return uCloned.String()
}

func httpGetWithHeaders(httpClient *http.Client, url string, httpHeaders [][]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range httpHeaders {
		req.Header.Add(kv[0], kv[1])
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("status=%d", res.StatusCode)
	}
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if err := res.Body.Close(); err != nil {
		return nil, err
	}
	return bodyBytes, nil
}

func pipingPostJson(httpClient *http.Client, url string, httpHeaders [][]string, jsonBytes []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for _, kv := range httpHeaders {
		req.Header.Add(kv[0], kv[1])
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("status=%d", res.StatusCode)
	}
	if _, err := io.Copy(io.Discard, res.Body); err != nil {
		return err
	}
	if err := res.Body.Close(); err != nil {
		return err
	}
	return nil
}

func sendSdp(logger *log.Logger, httpClient *http.Client, pipingServerUrl *url.URL, httpHeaders [][]string, localId string, remoteId string, description *webrtc.SessionDescription) error {
	jsonBytes, err := json.Marshal(description)
	if err != nil {
		return err
	}
	url := urlJoin(pipingServerUrl, fmt.Sprintf("%s-%s/sdp", localId, remoteId))
	logger.Printf("sending sdp %s to %s...", string(jsonBytes), url)
	return pipingPostJson(httpClient, url, httpHeaders, jsonBytes)
}

func receiveSdp(logger *log.Logger, httpClient *http.Client, pipingServerUrl *url.URL, httpHeaders [][]string, localId string, remoteId string) (*webrtc.SessionDescription, error) {
	url := urlJoin(pipingServerUrl, fmt.Sprintf("%s-%s/sdp", remoteId, localId))
	logger.Printf("receiving sdp from %s ...", url)
	sdpBytes, err := httpGetWithHeaders(httpClient, url, httpHeaders)
	if err != nil {
		return nil, err
	}
	sdp := webrtc.SessionDescription{}
	if err := json.Unmarshal(sdpBytes, &sdp); err != nil {
		return nil, err
	}
	return &sdp, nil
}

func sendCandidates(logger *log.Logger, httpClient *http.Client, pipingServerUrl *url.URL, httpHeaders [][]string, localId string, remoteId string, cs []*webrtc.ICECandidate) error {
	var candidateJsons []webrtc.ICECandidateInit
	for _, c := range cs {
		candidateJsons = append(candidateJsons, c.ToJSON())
	}
	candidateBytes, err := json.Marshal(&candidateJsons)
	if err != nil {
		return err
	}
	if len(cs) == 0 {
		// https://github.com/golang/go/issues/31811
		candidateBytes = []byte("[]")
	}
	logger.Printf("sending candidates %s...", string(candidateBytes))
	return pipingPostJson(httpClient, urlJoin(pipingServerUrl, fmt.Sprintf("%s-%s/candidates", localId, remoteId)), httpHeaders, candidateBytes)
}

func receiveCandidates(httpClient *http.Client, pipingServerUrl *url.URL, httpHeaders [][]string, localId string, remoteId string) ([]webrtc.ICECandidateInit, error) {
	candidateBytes, err := httpGetWithHeaders(httpClient, urlJoin(pipingServerUrl, fmt.Sprintf("%s-%s/candidates", remoteId, localId)), httpHeaders)
	if err != nil {
		return nil, err
	}
	var candidates []webrtc.ICECandidateInit
	if err := json.Unmarshal(candidateBytes, &candidates); err != nil {
		return nil, err
	}
	return candidates, nil
}
