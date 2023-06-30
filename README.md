# filetransfer



## Introduction

*filetransfer* is the file transfer using gRPC streaming written in Go.

> One of the unusually nice features of the gRPC system is that it
> supports streaming of messages. Although each individual message
> in a stream cannot be more than 1-2MB (or else we get strange EOF
> failures back from the gRPC library), we can readily transfer
> any large number of 1MB chunks, resulting in a large file
> transfer.
>
> In short, this library demonstrates how to use gRPC http://www.grpc.io/ in
> Golang (Go) to stream large files (of arbitrary size) between
> two endpoints.
>
> For security, we demonstrate two choices. We show how to do the
> transfer using TLS and using an embedded SSH tunnel. The ssh client and server are
> compiled into the binaries. You don't need to run a separate sshd on
> your host or docker container.
>
> Under flag `-ssh` to both client and server, we setup an SSH tunnel using https://github.com/glycerine/sshego
> and 4096-bit RSA keys. Key exchange is done with `kexAlgoCurve25519SHA256`.
>
> For verifying the integrity of the transfer, we use Blake2B cryptographic hashes (https://blake2.net/) on both the inidividual chunks, and on the complete, cumulative transfer.



## Prerequisites

- Go >= 1.18.0
- protoc >= 3.0.0



## Build

```bash
pushd server
./script/build.sh
popd

pushd client
./script/build.sh
popd
```



## Run

### Run with TLS

```bash
# Run server in a separate terminal
pushd server
./bin/server
popd

# Run client in a separate terminal
pushd client
./bin/client
popd
```



### Run with SSH

> First a user account with a public/private key pair must be generated, then the server's host key must be accepted and stored.
>
> Copy $HOME/.ssh/id_rsa from the server host to the client host (assuming they are distinct hosts, keep the same directory structure).

```bash
# Run server in a separate terminal
pushd server
# Generated account
./bin/server -adduser $USER

# Start server
./bin/server -ssh
popd

# Run client in a separate terminal
pushd client
# Store server's host key
./bin/client -ssh -new

# Start client
./bin/client -ssh
popd
```



## License

MIT License



## Author

Jason E. Aten, Ph.D.
