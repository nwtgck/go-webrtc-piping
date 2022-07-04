package core

import (
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

type NetworkType int64

const (
	NetworkTypeTcp NetworkType = iota
	NetworkTypeUdp
)

func createConfig() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				// TODO: hard code
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
}

func NewDetachablePeerConnection(configuration webrtc.Configuration) (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	i := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		return nil, err
	}

	s := webrtc.SettingEngine{}
	s.DetachDataChannels()

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i), webrtc.WithSettingEngine(s))
	//api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))
	return api.NewPeerConnection(configuration)
	//return webrtc.NewPeerConnection(configuration)
}

func offerSideId(path string) string {
	return "offer_" + path
}

func answerSideId(path string) string {
	return "answer_" + path
}
