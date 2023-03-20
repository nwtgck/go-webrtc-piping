package duplex

import (
	"github.com/pion/interceptor"
	"github.com/pion/webrtc/v3"
	"io"
	"log"
	"os"
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

func stdinToDataChannel(logger *log.Logger, dataChannel *webrtc.DataChannel) error {
	var buf [32 * 1024]byte // same size as io.Copy()
	for {
		n, err := os.Stdin.Read(buf[:])
		if err == io.EOF {
			logger.Printf("finish: stdin -> data channel")
			if err := dataChannel.Send([]byte{}); err != nil {
				logger.Printf("send err: %v", err)
				return err
			}
			return nil
		}
		if err != nil {
			logger.Printf("read stdin err: %v", err)
			return err
		}
		// empty bytes means finish
		if n == 0 {
			continue
		}
		if err = dataChannel.Send(buf[:n]); err != nil {
			logger.Printf("send err: %v", err)
			return err
		}
	}
}

func registerOnMessageForDataChannelToStdout(logger *log.Logger, dataChannel *webrtc.DataChannel) <-chan error {
	errCh := make(chan error)
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		if len(msg.Data) == 0 {
			logger.Printf("finish: data channel -> stdout")
			errCh <- nil
		}
		_, err := os.Stdout.Write(msg.Data)
		if err != nil {
			logger.Printf("write stdout err: %v", err)
			errCh <- err
		}
		msg.Data = nil
	})
	return errCh
}
