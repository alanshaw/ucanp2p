package ucanp2p

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/alanshaw/ucanp2p/internal/testutil"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	ipldschema "github.com/ipld/go-ipld-prime/schema"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/storacha/go-ucanto/client"
	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/invocation"
	"github.com/storacha/go-ucanto/core/receipt"
	"github.com/storacha/go-ucanto/core/receipt/fx"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/core/schema"
	"github.com/storacha/go-ucanto/principal"
	ed25519 "github.com/storacha/go-ucanto/principal/ed25519/signer"
	userver "github.com/storacha/go-ucanto/server"
	"github.com/storacha/go-ucanto/ucan"
	"github.com/storacha/go-ucanto/validator"
	"github.com/stretchr/testify/require"
)

func echoTS() *ipldschema.TypeSystem {
	ts, err := ipld.LoadSchemaBytes([]byte(`
    type EchoCaveats struct {
      message String
    }
    type EchoOk struct {
      message String
    }
    type EchoFailure any
  `))
	if err != nil {
		panic(fmt.Errorf("loading echo schema"))
	}
	return ts
}

type echoCaveats struct {
	Message string
}

func (ec echoCaveats) ToIPLD() (ipld.Node, error) {
	return bindnode.Wrap(&ec, echoTS().TypeByName("EchoCaveats")), nil
}

type echoOk = echoCaveats

var echo = validator.NewCapability(
	"cave/echo",
	schema.DIDString(),
	schema.Struct[echoCaveats](echoTS().TypeByName("EchoCaveats"), nil),
	nil,
)

func TestEcho(t *testing.T) {
	serverID := testutil.Must(ed25519.Generate())(t)
	serverPriv := toCryptoPriv(t, serverID)

	serverHost := testutil.Must(libp2p.New(libp2p.Identity(serverPriv)))(t)
	defer serverHost.Close()

	// setup HTTP request listener on the libp2p host
	listener, _ := NewHTTPListener(serverHost)
	defer listener.Close()
	go func() {
		// define the ucanto server
		server := testutil.Must(userver.NewServer(
			serverID,
			userver.WithServiceMethod(
				echo.Can(),
				userver.Provide(echo, func(ctx context.Context, cap ucan.Capability[echoCaveats], inv invocation.Invocation, iCtx userver.InvocationContext) (echoCaveats, fx.Effects, error) {
					fmt.Printf("echoing: %s\n", cap.Nb().Message)
					return echoCaveats{Message: cap.Nb().Message}, nil, nil
					// return echoCaveats{}, nil, errors.New("boom")
				}),
			),
		))(t)
		// handle request
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			res, _ := server.Request(r.Context(), NewHTTPRequest(r.Body, r.Header))
			for key, vals := range res.Headers() {
				for _, v := range vals {
					w.Header().Add(key, v)
				}
			}
			if res.Status() != 0 {
				w.WriteHeader(res.Status())
			}
			testutil.Must(io.Copy(w, res.Body()))(t)
		})
		httpServer := &http.Server{}
		httpServer.Serve(listener) // Note: use the libp2p listener!
	}()

	clientID := testutil.Must(ed25519.Generate())(t)
	clientPriv := toCryptoPriv(t, clientID)
	proof := delegation.FromDelegation(testutil.Must(echo.Delegate(serverID, clientID, serverID.DID().String(), echoCaveats{}))(t))

	clientHost := testutil.Must(libp2p.New(libp2p.Identity(clientPriv)))(t)
	defer clientHost.Close()

	// HTTP over p2p transport and CAR encoding
	channel := NewHTTPChannel(clientHost, serverHost.Peerstore().PeerInfo(serverHost.ID()), "/")
	conn := testutil.Must(client.NewConnection(serverID, channel))(t)

	// the message to echo
	message := "Hello World!"

	// create invocation to perform the echo task with granted capabilities
	cap := echo.New(serverID.DID().String(), echoCaveats{Message: message})
	inv := testutil.Must(invocation.Invoke(clientID, serverID, cap, delegation.WithProof(proof)))(t)

	// send the invocation to the server
	resp := testutil.Must(client.Execute(t.Context(), []invocation.Invocation{inv}, conn))(t)

	// create new receipt reader
	rcptReader := testutil.Must(receipt.NewReceiptReaderFromTypes[echoOk, ipld.Node](echoTS().TypeByName("EchoOk"), echoTS().TypeByName("EchoFailure")))(t)

	// get the receipt link for the invocation from the response
	rcptlnk, ok := resp.Get(inv.Link())
	require.True(t, ok)
	// read the receipt for the invocation from the response
	rcpt := testutil.Must(rcptReader.Read(rcptlnk, resp.Blocks()))(t)

	result.MatchResultR0(rcpt.Out(), func(o echoOk) {
		require.Equal(t, message, o.Message)
	}, func(x ipld.Node) {
		err := testutil.BindFailure(t, x)
		require.Nil(t, err)
	})
}

func toCryptoPriv(t *testing.T, s principal.Signer) crypto.PrivKey {
	require.Equal(t, uint64(ed25519.Code), s.Code())
	pk, err := crypto.UnmarshalEd25519PrivateKey(s.Raw())
	require.NoError(t, err)
	return pk
}
