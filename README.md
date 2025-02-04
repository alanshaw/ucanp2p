# ucanp2p

Ucanto channels for libp2p.

## Install

```sh
go get github.com/alanshaw/ucanp2p
```

## Usage

### Client

```go
package main

import (
	"github.com/alanshaw/ucanp2p"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	local := libp2p.New()
	remote, _ := peer.AddrInfoFromString("/p2p/12D3KooWLEX6SeH6KJg4wssWqHiZci4yDKr1D9fuVeVR4TYDDYHt")

	// Create a new HTTP over p2p channel (uses go-libp2p-http)
	channel := ucanp2p.NewHTTPChannel(local, remote, "/")

	// use channel with client (see https://github.com/storacha/go-ucanto#client)
}
```

### Server

```go
package main

import (
	"io"
	"net/http"

	"github.com/alanshaw/ucanp2p"
	"github.com/libp2p/go-libp2p"
	"github.com/storacha/go-ucanto/server"
)

func main() {
	host := libp2p.New()
	listener, _ := ucanp2p.NewHTTPListener(host)
	defer listener.Close()

	var server server.ServerView

	// init your server (see https://github.com/storacha/go-ucanto#server)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		res, err := server.Request(ucanp2p.NewHTTPRequest(r.Body, r.Header))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for key, vals := range res.Headers() {
			for _, v := range vals {
				w.Header().Add(key, v)
			}
		}
		if res.Status() != 0 {
			w.WriteHeader(res.Status())
		}
		_, err = io.Copy(w, res.Body())
		// TODO: log error?
	})
	httpServer := &http.Server{}
	httpServer.Serve(listener) // Note: use the libp2p listener!
}
```

## Contributing

Feel free to join in. All welcome. [Open an issue](https://github.com/alanshaw/ucanp2p/issues)!

## License

Dual-licensed under [MIT or Apache 2.0](https://github.com/alanshaw/ucanp2p/blob/main/LICENSE.md)
