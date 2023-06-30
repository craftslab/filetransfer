package config

import (
	"github.com/glycerine/idem"

	"github.com/craftslab/filetransfer/server/api"
	_grpc "github.com/craftslab/filetransfer/server/grpc"
)

func NewServerConfig(myID string) *_grpc.ServerConfig {
	return &_grpc.ServerConfig{
		MyID:                myID,
		ServerGotGetReply:   make(chan *api.BcastGetReply),
		ServerGotSetRequest: make(chan *api.BcastSetRequest),
		Halt:                idem.NewHalter(),
	}
}
