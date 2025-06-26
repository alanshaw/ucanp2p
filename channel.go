package ucanp2p

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	gostream "github.com/libp2p/go-libp2p-gostream"
	p2phttp "github.com/libp2p/go-libp2p-http"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/storacha/go-ucanto/transport"
	uhttp "github.com/storacha/go-ucanto/transport/http"
)

var NewHTTPRequest = uhttp.NewHTTPRequest
var NewHTTPResponse = uhttp.NewHTTPResponse
var NewHTTPError = uhttp.NewHTTPError

const HTTPProtocol = protocol.ID("/libp2p-http/ucanto")

type httpchannel struct {
	client host.Host
	peer   peer.AddrInfo
	path   string
}

func (c *httpchannel) Request(ctx context.Context, req transport.HTTPRequest) (transport.HTTPResponse, error) {
	path := c.path
	if !strings.HasPrefix(path, "/") {
		path = fmt.Sprintf("/%s", path)
	}

	hr, err := http.NewRequest("POST", fmt.Sprintf("libp2p://%s%s", c.peer.ID, path), req.Body())
	if err != nil {
		return nil, fmt.Errorf("creating HTTP request: %s", err)
	}
	hr.Header = req.Headers()

	err = c.client.Connect(ctx, c.peer)
	if err != nil {
		return nil, fmt.Errorf("connecting to remote peer: %w", err)
	}
	rt := p2phttp.NewTransport(c.client, p2phttp.ProtocolOption(HTTPProtocol))

	res, err := rt.RoundTrip(hr)
	if err != nil {
		return nil, fmt.Errorf("doing HTTP request: %s", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, NewHTTPError(fmt.Sprintf("HTTP Request failed. %s %s → %d", hr.Method, hr.URL, res.StatusCode), res.StatusCode, res.Header)
	}

	return NewHTTPResponse(res.StatusCode, res.Body, res.Header), nil
}

// NewHTTPChannel creates a new HTTP over libp2p channel.
func NewHTTPChannel(client host.Host, peer peer.AddrInfo, path string) transport.Channel {
	return &httpchannel{client, peer, path}
}

// NewHTTPListener creates a new [net.Listener] that listens to
// "/libp2p-http/ucanto" protocol messages sent to the server host.
func NewHTTPListener(server host.Host) (net.Listener, error) {
	return gostream.Listen(server, HTTPProtocol)
}
