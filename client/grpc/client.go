// gRPC client
package grpc

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"log"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/devops-filetransfer/blake2b"

	"github.com/devops-filetransfer/filetransfer/client/print"
	pb "github.com/devops-filetransfer/filetransfer/client/protobuf"
)

type client struct {
	hasher     hash.Hash
	nextChunk  int64
	peerClient pb.PeerClient
}

func NewClient(conn *grpc.ClientConn) *client {
	h, err := blake2b.New(nil)
	print.PanicOn(err)

	return &client{
		hasher:     h,
		peerClient: pb.NewPeerClient(conn),
	}
}

func (c *client) startNewFile() {
	c.hasher.Reset()
	c.nextChunk = 0
}

func (c *client) RunSendFile(path string, data []byte, maxChunkSize int, isBcastSet bool, myID string) error {
	startOfRunSendFile := time.Now().UTC()
	startOfRunSendFileNanoUint64 := uint64(startOfRunSendFile.UnixNano())

	c.startNewFile()
	stream, err := c.peerClient.SendFile(context.Background())
	if err != nil {
		log.Fatalf("%v.SendFile(_) = _, %v", c.peerClient, err)
	}

	n := len(data)
	numChunk := n / maxChunkSize
	if n%maxChunkSize > 0 {
		numChunk++
	}

	nextByte := 0
	lastChunk := numChunk - 1

	for i := 0; i < numChunk; i++ {
		sendLen := intMin(maxChunkSize, n-(i*maxChunkSize))
		chunk := data[nextByte:(nextByte + sendLen)]
		nextByte += sendLen

		var nk pb.BigFileChunk

		nk.IsBcastSet = isBcastSet
		nk.Filepath = path
		nk.SizeInBytes = int64(sendLen)
		nk.SendTime = uint64(time.Now().UnixNano())
		nk.OriginalStartSendTime = startOfRunSendFileNanoUint64

		// checksums
		c.hasher.Write(chunk)
		nk.Blake2B = blake2bOfBytes(chunk)
		nk.Blake2BCumulative = []byte(c.hasher.Sum(nil))

		nk.Data = chunk
		nk.ChunkNumber = c.nextChunk
		c.nextChunk++
		nk.IsLastChunk = (i == lastChunk)

		if err := stream.Send(&nk); err != nil {
			if err == io.EOF {
				if !nk.IsLastChunk {
					panic(fmt.Sprintf("'%s' we got io.EOF before "+
						"the last chunk! At: %v of %v", path, nk.ChunkNumber, numChunk))
				} else {
					break
				}
			}
			panic(err)
		}
	}

	reply, err := stream.CloseAndRecv()
	if err != nil {
		log.Printf("%v.CloseAndRecv() got error %v, want %v. reply=%v", stream, err, nil, reply)
		return err
	}

	compared := bytes.Compare(reply.WholeFileBlake2B, []byte(c.hasher.Sum(nil)))
	log.Printf("%s client.runSendFile got from stream.CloseAndRecv() a Reply with checksum: '%x'; checksum matches the sent data: %v; size sent = %v, size received = %v. startOfRunSendFile='%v'.", myID, reply.WholeFileBlake2B, compared == 0, len(data), reply.SizeInBytes, startOfRunSendFile)

	if int64(len(data)) != reply.SizeInBytes {
		panic("size mismatch")
	}

	return nil
}

func blake2bOfBytes(by []byte) []byte {
	h, err := blake2b.New(nil)
	print.PanicOn(err)

	h.Write(by)

	return []byte(h.Sum(nil))
}

func intMin(a, b int) int {
	if a < b {
		return a
	}

	return b
}
