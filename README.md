# go-webrtc-piping
[![CI](https://github.com/nwtgck/go-webrtc-piping/actions/workflows/ci.yml/badge.svg)](https://github.com/nwtgck/go-webrtc-piping/actions/workflows/ci.yml)

WebRTC P2P tunneling/duplex with [Piping Server](https://github.com/nwtgck/piping-server) WebRTC signaling

## Install for Windows
[Download](https://github.com/nwtgck/go-webrtc-piping/releases/download/v0.3.0/webrtc-piping-0.3.0-windows-amd64.zip)

## Install for macOS
```bash
brew install nwtgck/webrtc-piping/webrtc-piping
```

## Install for Ubuntu
```bash
wget https://github.com/nwtgck/go-webrtc-piping/releases/download/v0.3.0/webrtc-piping-0.3.0-linux-amd64.deb
sudo dpkg -i webrtc-piping-0.3.0-linux-amd64.deb 
```

Get more executables in the [releases](https://github.com/nwtgck/go-webrtc-piping/releases).

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

## Without ICE servers

Specify `--ice-servers='[]'`.

## Help

```
WebRTC tunnel with Piping Server WebRTC signaling

Usage:
  webrtc-piping [flags]
  webrtc-piping [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  duplex      Duplex communication
  help        Help about any command
  tunnel      Tunneling TCP or UDP

Flags:
      --dns-server string    DNS server (e.g. 1.1.1.1:53)
  -H, --header stringArray   HTTP header
  -h, --help                 help for webrtc-piping
      --ice-servers json     ICE servers (default [{"urls":["stun:stun.l.google.com:19302"]}])
  -k, --insecure             Allow insecure server connections when using SSL
  -s, --server string        Piping Server URL (default "https://ppng.io")
  -v, --verbose              verbose output
  -V, --version              show version

Use "webrtc-piping [command] --help" for more information about a command.
```
