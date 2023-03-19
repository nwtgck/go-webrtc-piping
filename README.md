# go-webrtc-piping
WebRTC P2P tunneling/duplex with Piping Server WebRTC signaling

## TCP tunneling

The following command forwards 8888 port to 9999 port.   

```bash
webrtc-piping tunnel 8888 mypath 
```

```bash
webrtc-piping tunnel -l 9999 mypath
```

## UDP tunneling

Adding -u or --udp option forwards UDP port.

```bash
webrtc-piping tunnel -u 8888 mypath 
```

```bash
webrtc-piping tunnel -ul 9999 mypath
```

## Full-duplex

```bash
echo hello1 | webrtc-piping duplex mypath1 mypath2
# => hello2
```

```bash
echo hello2 | webrtc-piping duplex mypath2 mypath1
# => hello1
```
