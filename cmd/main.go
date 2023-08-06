package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/a-h/gemini"
)

var Version = ""

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "request":
		request(os.Args[2:])
		return
	case "serve":
		serve(os.Args[2:])
		return
	case "version":
		fmt.Println(Version)
		return
	case "--version":
		fmt.Println(Version)
		return
	}
	usage()
}

func usage() {
	fmt.Println(`usage: gemini <command> [parameters]
To see help text, you can run:

  gemini request --help
  gemini serve --help
  gemini version

examples:

  gemini request --insecure --verbose gemini://example.com/pass
  gemini serve --domain=example.com --certFile=server.crt --keyFile=server.key --path=.`)
	os.Exit(1)
}

func request(args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Shutting down...")
		cancel()
	}()

	cmd := flag.NewFlagSet("request", flag.ExitOnError)
	noTLSFlag := cmd.Bool("noTLS", false, "Don't connect with TLS, or send client server certificates.")
	insecureFlag := cmd.Bool("insecure", false, "Allow any server certificate.")
	certFileFlag := cmd.String("certFile", "", "Path to a client certificate file (must also set keyFile if this is used).")
	keyFileFlag := cmd.String("keyFile", "", "Path to a client key file (must also set certFile if this is used).")
	verboseFlag := cmd.Bool("verbose", false, "Print both headers and body.")
	headersFlag := cmd.Bool("headers", false, "Print only the headers.")
	allowBinaryFlag := cmd.Bool("allowBinary", false, "Set to true to enable printing binary to the console.")
	readTimeoutFlag := cmd.Duration("readTimeout", time.Second*5, "Set the duration, e.g. 1m or 5s.")
	writeTimeoutFlag := cmd.Duration("writeTimeout", time.Second*5, "Set the duration, e.g. 1m or 5s.")
	helpFlag := cmd.Bool("help", false, "Print help and exit.")
	err := cmd.Parse(args)
	if err != nil || *helpFlag {
		cmd.PrintDefaults()
		return
	}
	urlString := strings.Join(cmd.Args(), "")
	if urlString == "" {
		cmd.PrintDefaults()
		return
	}
	u, err := url.Parse(urlString)
	if err != nil {
		fmt.Printf("Failed to parse gemini URL %q: %v\n", urlString, err)
		os.Exit(1)
	}

	client := gemini.NewClient()
	client.ReadTimeout = *readTimeoutFlag
	client.WriteTimeout = *writeTimeoutFlag
	if *insecureFlag {
		client.Insecure = true
	}
	if *certFileFlag != "" {
		keyPair, err := tls.LoadX509KeyPair(*certFileFlag, *keyFileFlag)
		if err != nil {
			fmt.Printf("Failed to parse certFile / keyFile: %v\n", err)
			os.Exit(1)
		}
		client.AddClientCertificate("/", keyPair)
	}
	var authenticated, ok bool
	var resp *gemini.Response
	var certificates []string
	if *noTLSFlag {
		ok = true // No server validation takes place.
		resp, err = client.RequestNoTLS(ctx, u)
	} else {
		resp, certificates, authenticated, ok, err = client.RequestURL(ctx, u)
	}
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		os.Exit(1)
	}
	if !ok && !*insecureFlag {
		fmt.Println("Unexpected certificates provided by server.")
		for _, c := range certificates {
			fmt.Println(" ", c)
		}
		os.Exit(1)
	}
	if *certFileFlag != "" && !authenticated {
		fmt.Println("Authentication failed, the certificate was rejected by the server.")
		os.Exit(1)
	}
	if *verboseFlag || *headersFlag {
		fmt.Printf("%v %v\r\n", resp.Header.Code, resp.Header.Meta)
	}
	if *headersFlag != true && !gemini.IsErrorCode(resp.Header.Code) {
		if strings.HasPrefix(resp.Header.Meta, "text/") {
			s := bufio.NewScanner(resp.Body)
			for s.Scan() {
				fmt.Println(s.Text())
			}
			if s.Err() != nil {
				fmt.Printf("Error reading response body: %v\n", s.Err())
				os.Exit(1)
			}
			defer resp.Body.Close()
		} else if *allowBinaryFlag {
			_, err := io.Copy(os.Stdout, resp.Body)
			if err != nil {
				fmt.Printf("Error reading binary response body: %v\n", err)
				os.Exit(1)
			}
			defer resp.Body.Close()
		} else {
			fmt.Println("Binary output skipped, set allowBinary to allow.")
			os.Exit(1)
		}
	}
	if gemini.IsErrorCode(resp.Header.Code) {
		os.Exit(1)
	}
}

func newServerConfig() serverConfig {
	return serverConfig{
		Domain:       make(map[string]domainConfig),
		Port:         1965,
		ReadTimeout:  time.Second * 5,
		WriteTimeout: time.Second * 10,
	}
}

type serverConfig struct {
	Domain       map[string]domainConfig
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type domainConfig struct {
	Path         string
	CertFilePath string
	KeyFilePath  string
}

func (dc domainConfig) IsValid(name string) error {
	var errs []error
	if dc.Path == "" {
		errs = append(errs, fmt.Errorf("%s: no path configured", name))
	}
	if dc.CertFilePath == "" {
		errs = append(errs, fmt.Errorf("%s: no cert file configured", name))
	}
	if dc.KeyFilePath == "" {
		errs = append(errs, fmt.Errorf("%s: no key file configured", name))
	}
	return errors.Join(errs...)
}

var errNoDomainsConfigured = errors.New("no domains configured")

func (sc serverConfig) IsValid() error {
	var errs []error
	if len(sc.Domain) == 0 {
		return errNoDomainsConfigured
	}
	for name, dc := range sc.Domain {
		errs = append(errs, dc.IsValid(name))
	}
	return errors.Join(errs...)
}

func loadConfigFile(conf io.Reader) (serverConfig serverConfig, err error) {
	_, err = toml.NewDecoder(conf).Decode(&serverConfig)
	if err != nil {
		return
	}
	if serverConfig.Port == 0 {
		serverConfig.Port = defaultPort
	}
	if serverConfig.ReadTimeout == 0 {
		serverConfig.ReadTimeout = defaultReadTimeout
	}
	if serverConfig.WriteTimeout == 0 {
		serverConfig.WriteTimeout = defaultWriteTimeout
	}
	return serverConfig, serverConfig.IsValid()
}

var (
	defaultReadTimeout  = time.Second * 5
	defaultWriteTimeout = time.Second * 10
	defaultPort         = 1965
	defaultPath         = "."
)

func serve(args []string) {
	// Parse flags.
	cmd := flag.NewFlagSet("serve", flag.ExitOnError)
	certFileFlag := cmd.String("certFile", "", "(required) Path to a server certificate file (must also set keyFile if this is used).")
	keyFileFlag := cmd.String("keyFile", "", "(required) Path to a server key file (must also set certFile if this is used).")
	domainFlag := cmd.String("domain", "localhost", "The domain to listen on.")
	pathFlag := cmd.String("path", defaultPath, "Path containing content.")
	portFlag := cmd.Int("port", defaultPort, "Address to listen on.")
	readTimeoutFlag := cmd.Duration("readTimeout", defaultReadTimeout, "Set the duration, e.g. 1m or 5s.")
	writeTimeoutFlag := cmd.Duration("writeTimeout", defaultWriteTimeout, "Set the duration, e.g. 1m or 5s.")
	configPathFlag := cmd.String("config", "", "Path to a TOML config file.")
	helpFlag := cmd.Bool("help", false, "Print help and exit.")

	// Print defaults.
	err := cmd.Parse(args)
	if err != nil || *helpFlag {
		cmd.PrintDefaults()
		return
	}

	// Load config.
	serverConfig := newServerConfig()
	if *configPathFlag != "" {
		r, err := os.Open(*configPathFlag)
		if err != nil {
			fmt.Printf("error: invalid config path: %v\n", err)
			os.Exit(1)
		}
		serverConfig, err = loadConfigFile(r)
		if err != nil {
			fmt.Printf("error: invalid config: %v\n", err)
			os.Exit(1)
		}
	} else {
		if *certFileFlag == "" || *keyFileFlag == "" {
			fmt.Println("error: require certFile and keyFile flags to create server")
			fmt.Println()
			cmd.PrintDefaults()
			os.Exit(1)
		}
		serverConfig.Port = *portFlag
		serverConfig.ReadTimeout = *readTimeoutFlag
		serverConfig.WriteTimeout = *writeTimeoutFlag
		serverConfig.Domain[*domainFlag] = domainConfig{
			Path:         *pathFlag,
			CertFilePath: *certFileFlag,
			KeyFilePath:  *keyFileFlag,
		}
	}

	// Create handlers.
	domainToHandler := make(map[string]*gemini.DomainHandler)
	for domain, config := range serverConfig.Domain {
		h := gemini.FileSystemHandler(gemini.Dir(config.Path))
		cert, err := tls.LoadX509KeyPair(config.CertFilePath, config.KeyFilePath)
		if err != nil {
			fmt.Printf("error: failed to load certificates for domain %q: %v\n", domain, err)
			os.Exit(1)
		}
		dh := gemini.NewDomainHandler(domain, cert, h)
		domainToHandler[strings.ToLower(domain)] = dh
	}

	// Start server.
	ctx := context.Background()
	server := gemini.NewServer(ctx, fmt.Sprintf(":%d", serverConfig.Port), domainToHandler)
	server.ReadTimeout = serverConfig.ReadTimeout
	server.WriteTimeout = serverConfig.WriteTimeout
	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
