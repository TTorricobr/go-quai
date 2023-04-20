// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package ethclient provides a client for the Ethereum RPC API.
package quaiclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dominant-strategies/go-quai/common"
	"github.com/dominant-strategies/go-quai/core/types"
	"github.com/dominant-strategies/go-quai/log"
	"github.com/dominant-strategies/go-quai/rpc"
)

var exponentialBackoffCeilingSecs int64 = 60 // 1 minute

// Client defines typed wrappers for the Ethereum RPC API.
type Client struct {
	c *rpc.Client
}

// Dial connects a client to the given URL.
func Dial(rawurl string) (*Client, error) {
	return DialContext(context.Background(), rawurl)
}

func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	connectStatus := false
	attempts := 0

	var c *rpc.Client
	var err error
	for !connectStatus {
		c, err = rpc.DialContext(ctx, rawurl)
		if err == nil {
			break
		}

		attempts += 1
		// exponential back-off implemented
		// delaySecs := int64(math.Floor((math.Pow(2, float64(attempts)) - 1) * 0.5))
		delaySecs := int64(1)
		if delaySecs > exponentialBackoffCeilingSecs {
			return nil, err
		}

		// should only get here if the ffmpeg record stream process dies
		log.Warn("Attempting to connect to go-quai node. Waiting and retrying...", "attempts", attempts, "delay", delaySecs, "url", rawurl)

		time.Sleep(time.Duration(delaySecs) * time.Second)
	}

	return NewClient(c), nil
}

// NewClient creates a client that uses the given RPC client.
func NewClient(c *rpc.Client) *Client {
	return &Client{c}
}

func (ec *Client) Close() {
	ec.c.Close()
}

type Termini struct {
	Termini []common.Hash `json:"termini"`
}

type pendingEtxs struct {
	Etxs types.Transactions `json:"pendingEtxs"`
}

func (ec *Client) Append(ctx context.Context, header *types.Header, domPendingHeader *types.Header, domTerminus common.Hash, domOrigin bool, newInboundEtxs types.Transactions) (types.Transactions, error) {
	fields := map[string]interface{}{
		"header":           header.RPCMarshalHeader(),
		"domPendingHeader": domPendingHeader.RPCMarshalHeader(),
		"domTerminus":      domTerminus,
		"domOrigin":        domOrigin,
		"newInboundEtxs":   newInboundEtxs,
	}

	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, "quai_append", fields)
	if err != nil {
		return nil, err
	}

	// Decode header and transactions.
	var pEtxs pendingEtxs
	if err := json.Unmarshal(raw, &pEtxs); err != nil {
		return nil, err
	}

	return pEtxs.Etxs, nil
}

func (ec *Client) SubRelayPendingHeader(ctx context.Context, pendingHeader types.PendingHeader, location common.Location) {
	data := map[string]interface{}{"Header": pendingHeader.Header.RPCMarshalHeader()}
	data["Termini"] = pendingHeader.Termini
	data["Location"] = location

	ec.c.CallContext(ctx, nil, "quai_subRelayPendingHeader", data)
}

func (ec *Client) NewGenesisPendingHeader(ctx context.Context, header *types.Header) {
	ec.c.CallContext(ctx, nil, "quai_newGenesisPendingHeader", header.RPCMarshalHeader())
}

// GetManifest will get the block manifest ending with the parent hash
func (ec *Client) GetManifest(ctx context.Context, blockHash common.Hash) (types.BlockManifest, error) {
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, "quai_getManifest", blockHash)
	if err != nil {
		return nil, err
	}
	var manifest types.BlockManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func (ec *Client) SendPendingEtxsToDom(ctx context.Context, pEtxs types.PendingEtxs) error {
	fields := make(map[string]interface{})
	fields["header"] = pEtxs.Header.RPCMarshalHeader()
	fields["etxs"] = pEtxs.Etxs
	var raw json.RawMessage
	err := ec.c.CallContext(ctx, &raw, "quai_sendPendingEtxsToDom", fields)
	if err != nil {
		return err
	}
	return nil
}
