package piping_webrtc_signaling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"net/http"
)

type InitialJson struct {
	Version uint64 `json:"version"`
}

func sendSdp(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, localId string, remoteId string, description *webrtc.SessionDescription) error {
	payload, err := json.Marshal(description)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%s-%s/sdp", pipingServerUrl, localId, remoteId)
	logger.Printf("sending sdp to %s...", url)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for _, kv := range httpHeaders {
		req.Header.Add(kv[0], kv[1])
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	return nil
}

func receiveSdp(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, localId string, remoteId string) (*webrtc.SessionDescription, error) {
	url := fmt.Sprintf("%s/%s-%s/sdp", pipingServerUrl, remoteId, localId)
	logger.Printf("receiving sdp from %s ...", url)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range httpHeaders {
		req.Header.Add(kv[0], kv[1])
	}
	r, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	sdp := webrtc.SessionDescription{}
	if err := json.NewDecoder(r.Body).Decode(&sdp); err != nil {
		return nil, err
	}
	return &sdp, nil
}

func sendCandidate(logger *log.Logger, httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, localId string, remoteId string, c *webrtc.ICECandidate) error {
	candidateBytes, err := json.Marshal(c.ToJSON())
	if err != nil {
		return err
	}
	logger.Printf("sending candidate...")
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s-%s/candidate", pipingServerUrl, localId, remoteId), bytes.NewReader(candidateBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	for _, kv := range httpHeaders {
		req.Header.Add(kv[0], kv[1])
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	return nil
}

func receiveCandidate(httpClient *http.Client, pipingServerUrl string, httpHeaders [][]string, localId string, remoteId string) (*webrtc.ICECandidateInit, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s-%s/candidate", pipingServerUrl, remoteId, localId), nil)
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
	candidateBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if err := res.Body.Close(); err != nil {
		return nil, err
	}
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(candidateBytes, &candidate); err != nil {
		return nil, err
	}
	return &candidate, nil
}
