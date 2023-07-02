package config

import (
	"github.com/devops-filetransfer/filetransfer/server/api"
	_grpc "github.com/devops-filetransfer/filetransfer/server/grpc"
	"github.com/devops-filetransfer/idem"
)

func NewServerConfig(myID string) *_grpc.ServerConfig {
	return &_grpc.ServerConfig{
		MyID:                myID,
		ServerGotGetReply:   make(chan *api.BcastGetReply),
		ServerGotSetRequest: make(chan *api.BcastSetRequest),
		Halt:                idem.NewHalter(),
	}
}
