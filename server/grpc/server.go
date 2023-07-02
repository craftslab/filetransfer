// gRPC server
package grpc

import (
	"bytes"
	"flag"
	"fmt"
	"hash"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/devops-filetransfer/bchan"
	"github.com/devops-filetransfer/blake2b"
	"github.com/devops-filetransfer/filetransfer/server/api"
	"github.com/devops-filetransfer/filetransfer/server/exists"
	"github.com/devops-filetransfer/filetransfer/server/print"
	pb "github.com/devops-filetransfer/filetransfer/server/protobuf"
	"github.com/devops-filetransfer/idem"
	tun "github.com/devops-filetransfer/sshego"
)

type ServerConfig struct {
	MyID string
	Host string // ip address

	// by default, we use SSH
	UseTLS bool

	// For when your VPN already provides encryption.
	SkipEncryption bool // turn off both SSH and TLS.

	CertPath string
	KeyPath  string

	ExternalLsnPort int
	InternalLsnPort int
	CpuProfilePath  string

	SshegoCfg *tun.SshegoConfig

	ServerGotGetReply   chan *api.BcastGetReply
	ServerGotSetRequest chan *api.BcastSetRequest

	Halt *idem.Halter

	GrpcServer *grpc.Server
	Cls        *PeerServerClass
}

type PeerServerClass struct {
	lgs                api.LocalGetSet
	cfg                *ServerConfig
	GotFile            *bchan.Bchan
	mut                sync.Mutex
	filesReceivedCount int64
}

func NewPeerServerClass(lgs api.LocalGetSet, cfg *ServerConfig) *PeerServerClass {
	return &PeerServerClass{
		lgs:     lgs,
		cfg:     cfg,
		GotFile: bchan.New(1),
	}
}

func (s *PeerServerClass) IncrementGotFileCount() {
	s.mut.Lock()
	s.filesReceivedCount++
	count := s.filesReceivedCount
	s.mut.Unlock()

	s.GotFile.Bcast(count)
}

// Implement pb.PeerServer interface; the server is receiving a file here,
// because the client called SendFile() on the other end.
func (s *PeerServerClass) SendFile(stream pb.Peer_SendFileServer) error {
	var chunkCount int64
	path := ""
	var hasher hash.Hash

	log.Printf("%s peer.Server SendFile (for receiving a file) starting!", s.cfg.MyID)

	hasher, err := blake2b.New(nil)
	if err != nil {
		return err
	}

	var finalChecksum []byte
	const writeFileToDisk = false
	var fd *os.File
	var bytesSeen int64

	defer func() {
		if fd != nil {
			_ = fd.Close()
		}

		finalChecksum = []byte(hasher.Sum(nil))
		endTime := time.Now()

		log.Printf("%s this server.SendFile() call got %v chunks, byteCount=%v. with final checksum '%x'. defer running/is returning with err='%v'", s.cfg.MyID, chunkCount, bytesSeen, finalChecksum, err)

		errStr := ""
		if err != nil {
			errStr = err.Error()
		}

		sacErr := stream.SendAndClose(&pb.BigFileAck{
			Filepath:         path,
			SizeInBytes:      bytesSeen,
			RecvTime:         uint64(endTime.UnixNano()),
			WholeFileBlake2B: finalChecksum,
			Err:              errStr,
		})
		if sacErr != nil {
			log.Printf("warning: sacErr='%s' in gserv server.go PeerServerClass.SendFile() attempt to stream.SendAndClose().", sacErr)
		}
	}()

	firstChunkSeen := false
	var nk *pb.BigFileChunk

	for {
		nk, err = stream.Recv()
		if err == io.EOF {
			if nk != nil && len(nk.Data) > 0 {
				panic("we need to save this last chunk too!")
			}
			return nil
		}
		if err != nil {
			return err
		}

		// INVAR: we have a chunk
		if !firstChunkSeen {
			if nk.Filepath != "" {
				if writeFileToDisk {
					fd, err = os.Create(nk.Filepath + fmt.Sprintf("__%v", time.Now()))
					if err != nil {
						return err
					}
					defer func(fd *os.File) {
						_ = fd.Close()
					}(fd)
				}
			}
			firstChunkSeen = true
		}

		hasher.Write(nk.Data)
		cumul := hasher.Sum(nil)
		if 0 != bytes.Compare(cumul, nk.Blake2BCumulative) {
			return fmt.Errorf("cumulative checksums failed at chunk %v of '%s'. Observed: '%x', expected: '%x'.", nk.ChunkNumber, nk.Filepath, cumul, nk.Blake2BCumulative)
		}

		if path == "" {
			path = nk.Filepath
		}

		if path != "" && path != nk.Filepath {
			panic(fmt.Errorf("confusing between two different streams! '%s' vs '%s'", path, nk.Filepath))
		}

		if nk.SizeInBytes != int64(len(nk.Data)) {
			return fmt.Errorf("%v == nk.SizeInBytes != int64(len(nk.Data)) == %v", nk.SizeInBytes, int64(len(nk.Data)))
		}

		checksum := s.blake2bOfBytes(nk.Data)
		cmp := bytes.Compare(checksum, nk.Blake2B)
		if cmp != 0 {
			return fmt.Errorf("chunk %v bad .Data, checksum mismatch!",
				nk.ChunkNumber)
		}

		// INVAR: chunk passes tests, keep it.
		bytesSeen += int64(len(nk.Data))
		chunkCount++

		// TODO: user should store chunk somewhere here... or accumulate
		// all the chunks in memory
		// until ready to store it elsewhere; e.g. in boltdb.

		if writeFileToDisk {
			err = s.writeToFd(fd, nk.Data)
			if err != nil {
				return err
			}
		}

		if nk.IsLastChunk {
			return err
		}
	}
}

func (s *PeerServerClass) blake2bOfBytes(by []byte) []byte {
	h, err := blake2b.New(nil)
	print.PanicOn(err)

	h.Write(by)

	return h.Sum(nil)
}

func (s *PeerServerClass) writeToFd(fd *os.File, data []byte) error {
	w := 0
	n := len(data)

	for {
		nw, err := fd.Write(data[w:])
		if err != nil {
			return err
		}
		w += nw
		if nw >= n {
			return nil
		}
	}
}

func (c *ServerConfig) DefineFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.UseTLS, "tls", false, "Use TLS instead of the default SSH.")
	fs.BoolVar(&c.SkipEncryption, "skip-encryption", false, "Skip both TLS and SSH; for running on an already encrypted VPN.")
	fs.StringVar(&c.CertPath, "cert_file", "testdata/server1.pem", "The TLS cert file")
	fs.StringVar(&c.KeyPath, "key_file", "testdata/server1.key", "The TLS key file")
	fs.StringVar(&c.Host, "host", "127.0.0.1", "host IP address or name to bind")
	fs.IntVar(&c.ExternalLsnPort, "externalport", 10000, "The exteral server port")
	fs.IntVar(&c.InternalLsnPort, "iport", 10001, "The internal server port")
	fs.StringVar(&c.CpuProfilePath, "cpuprofile", "", "write cpu profile to file")
}

func (c *ServerConfig) ValidateConfig() error {
	if c.UseTLS {
		if c.KeyPath == "" {
			return fmt.Errorf("must provide -key_file under TLS")
		}

		if !exists.FileExists(c.KeyPath) {
			return fmt.Errorf("-key_path '%s' does not exist", c.KeyPath)
		}

		if c.CertPath == "" {
			return fmt.Errorf("must provide -key_file under TLS")
		}

		if !exists.FileExists(c.CertPath) {
			return fmt.Errorf("-cert_path '%s' does not exist", c.CertPath)
		}
	}

	if !c.UseTLS {
		lsn, err := net.Listen("tcp", fmt.Sprintf(":%v", c.InternalLsnPort))
		if err != nil {
			return fmt.Errorf("internal port %v already bound", c.InternalLsnPort)
		}
		_ = lsn.Close()
	}

	lsnX, err := net.Listen("tcp", fmt.Sprintf(":%v", c.ExternalLsnPort))
	if err != nil {
		return fmt.Errorf("external port %v already bound", c.ExternalLsnPort)
	}
	_ = lsnX.Close()

	return nil
}
