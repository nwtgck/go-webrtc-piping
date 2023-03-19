package duplex

import (
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
)

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
