package config

import (
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/craftslab/filetransfer/client/exists"
	"github.com/craftslab/filetransfer/client/ssh"
)

type ClientConfig struct {
	UseTLS bool

	// For when your VPN already provides encryption.
	SkipEncryption bool // turn off both SSH and TLS.

	AllowNewServer          bool // only give once to prevent MITM.
	TestAllowOneshotConnect bool
	CertPath                string
	KeyPath                 string
	ServerHost              string // ip address
	ServerPort              int
	ServerInternalHost      string // ip address
	ServerInternalPort      int
	ServerHostOverride      string

	Username             string
	PrivateKeyPath       string
	ClientKnownHostsPath string

	CpuProfilePath string

	PayloadSizeMegaBytes int
}

func (c *ClientConfig) DefineFlags(fs *flag.FlagSet) {
	fs.BoolVar(&c.AllowNewServer, "new", false, "allow new server host key to be recognized and stored in known-hosts")
	fs.BoolVar(&c.UseTLS, "tls", false, "Use TLS for security (default is SSH)")
	fs.BoolVar(&c.SkipEncryption, "skip-encryption", false, "Skip both TLS and SSH; for running on an already encrypted VPN.")
	fs.StringVar(&c.CertPath, "cert_file", "testdata/server1.pem", "The TLS cert file")
	fs.StringVar(&c.KeyPath, "key_file", "testdata/server1.key", "The TLS key file")
	fs.StringVar(&c.ServerHost, "host", "127.0.0.1", "host IP address or name to connect to")
	fs.IntVar(&c.ServerPort, "port", 10000, "The exteral server port")
	fs.StringVar(&c.ServerInternalHost, "ihost", "127.0.0.1", "internal host IP address or name to connect to")
	fs.IntVar(&c.ServerInternalPort, "iport", 10001, "The internal server port")
	fs.StringVar(&c.ServerHostOverride, "server_host_override", "x.test.youtube.com", "The server name use to verify the hostname returned by TLS handshake")

	user := os.Getenv("USER")
	fs.StringVar(&c.Username, "user", user, "username for sshd login (default is $USER)")

	home := os.Getenv("HOME")
	fs.StringVar(&c.PrivateKeyPath, "key", home+"/.ssh/.sshego.sshd.db/users/"+user+"/id_rsa", "private key for sshd login")
	fs.StringVar(&c.ClientKnownHostsPath, "known-hosts", home+"/.ssh/.sshego.cli.known.hosts", "path to our own known-hosts file, for sshd login")

	fs.StringVar(&c.CpuProfilePath, "cpuprofile", "", "write cpu profile to file")

	fs.IntVar(&c.PayloadSizeMegaBytes, "payload", 128, "transfer payload size in MB (megabytes)")
}

func (c *ClientConfig) ValidateConfig() error {
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

	return nil
}

func (c *ClientConfig) SetupTLS(opts *[]grpc.DialOption) {
	var sn string

	if c.ServerHostOverride != "" {
		sn = c.ServerHostOverride
	}

	var creds credentials.TransportCredentials

	if c.CertPath != "" {
		var err error
		creds, err = credentials.NewClientTLSFromFile(c.CertPath, sn)
		if err != nil {
			log.Fatalf("Failed to create TLS credentials %v", err)
		}
	} else {
		creds = credentials.NewClientTLSFromCert(nil, sn)
	}

	*opts = append(*opts, grpc.WithTransportCredentials(creds))
}

func (c *ClientConfig) SetupSSH(opts *[]grpc.DialOption) {
	destAddr := fmt.Sprintf("%v:%v", c.ServerInternalHost, c.ServerInternalPort)

	dialer, err := ssh.ClientSshMain(c.AllowNewServer, c.TestAllowOneshotConnect, c.PrivateKeyPath, c.ClientKnownHostsPath, c.Username, c.ServerHost, destAddr, int64(c.ServerPort))
	if err != nil {
		log.Fatalf("Failed to invoke clientSshMain %v", err)
	}

	*opts = append(*opts, grpc.WithContextDialer(dialer))

	// have to do this too, since we are using an SSH tunnel
	// that grpc doesn't know about:
	*opts = append(*opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
