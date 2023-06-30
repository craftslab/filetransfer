// gRPC server
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"runtime/pprof"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/craftslab/filetransfer/server/api"
	_grpc "github.com/craftslab/filetransfer/server/grpc"
	"github.com/craftslab/filetransfer/server/print"
	pb "github.com/craftslab/filetransfer/server/protobuf"
	"github.com/craftslab/filetransfer/server/ssh"
)

const ProgramName = "server"

func main() {

	myflags := flag.NewFlagSet(ProgramName, flag.ExitOnError)
	cfg := &_grpc.ServerConfig{}
	cfg.DefineFlags(myflags)
	cfg.SkipEncryption = true

	sshegoCfg := ssh.SetupSshFlags(myflags)

	args := os.Args[1:]
	if err := myflags.Parse(args); err != nil {
		log.Fatalf("%s parse args error: '%s'", ProgramName, err)
	}

	if cfg.CpuProfilePath != "" {
		f, err := os.Create(cfg.CpuProfilePath)
		if err != nil {
			log.Fatal(err)
		}
		_ = pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if err := cfg.ValidateConfig(); err != nil {
		log.Fatalf("%s command line flag error: '%s'", ProgramName, err)
	}

	var gRpcBindPort int
	var gRpcHost string
	if cfg.UseTLS {
		// use TLS
		gRpcBindPort = cfg.ExternalLsnPort
		gRpcHost = cfg.Host

		print.P("gRPC with TLS listening on %v:%v", gRpcHost, gRpcBindPort)

	} else if cfg.SkipEncryption {
		// no encryption at all
		gRpcBindPort = cfg.ExternalLsnPort
		gRpcHost = cfg.Host

	} else {
		// SSH will take the external, gRPC will take the internal.
		gRpcBindPort = cfg.InternalLsnPort
		gRpcHost = "127.0.0.1" // local only, behind the SSHD

		print.P("external SSHd listening on %v:%v, internal gRPC service listening on 127.0.0.1:%v", cfg.Host, cfg.ExternalLsnPort, cfg.InternalLsnPort)

	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%v:%d", gRpcHost, gRpcBindPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption

	if cfg.UseTLS {
		// use TLS
		creds, err := credentials.NewServerTLSFromFile(cfg.CertPath, cfg.KeyPath)
		if err != nil {
			log.Fatalf("Failed to generate credentials %v", err)
		}
		opts = []grpc.ServerOption{grpc.Creds(creds)}
	} else if cfg.SkipEncryption {
		// no encryption
		print.P("server configured to skip encryption.")
	} else {

		// use SSH
		err = ssh.ServerSshMain(sshegoCfg, cfg.Host,
			cfg.ExternalLsnPort, cfg.InternalLsnPort)
		print.PanicOn(err)
	}

	peer := NewPeerMemoryOnly()

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterPeerServer(grpcServer, _grpc.NewPeerServerClass(peer, cfg))
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to run grpcserver: %v", err)
	}
}

func NewPeerMemoryOnly() *PeerMemoryOnly {
	return &PeerMemoryOnly{}
}

type PeerMemoryOnly struct{}

func (peer *PeerMemoryOnly) LocalGet(key []byte, includeValue bool) (ki *api.KeyInv, err error) {
	return nil, fmt.Errorf("unimplimeneted")
}

func (peer *PeerMemoryOnly) LocalSet(ki *api.KeyInv) error {
	return nil
}
