package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

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
	if *headersFlag != true {
		s := bufio.NewScanner(resp.Body)
		for s.Scan() {
			fmt.Println(s.Text())
		}
		if s.Err() != nil {
			fmt.Printf("Error reading response body: %v\n", s.Err())
			os.Exit(1)
		}
		defer resp.Body.Close()
	}
	if gemini.IsErrorCode(resp.Header.Code) {
		os.Exit(1)
	}
}

func serve(args []string) {
	cmd := flag.NewFlagSet("serve", flag.ExitOnError)
	certFileFlag := cmd.String("certFile", "", "(required) Path to a server certificate file (must also set keyFile if this is used).")
	keyFileFlag := cmd.String("keyFile", "", "(required) Path to a server key file (must also set certFile if this is used).")
	domainFlag := cmd.String("domain", "localhost", "The domain to listen on.")
	pathFlag := cmd.String("path", ".", "Path containing content.")
	portFlag := cmd.Int("port", 1965, "Address to listen on.")
	helpFlag := cmd.Bool("help", false, "Print help and exit.")
	err := cmd.Parse(args)
	if err != nil || *helpFlag {
		cmd.PrintDefaults()
		return
	}
	if *certFileFlag == "" || *keyFileFlag == "" {
		fmt.Println("error: require certFile and keyFile flags to create server")
		fmt.Println()
		cmd.PrintDefaults()
		os.Exit(1)
	}
	h := gemini.FileSystemHandler(gemini.Dir(*pathFlag))

	cert, err := tls.LoadX509KeyPair(*certFileFlag, *keyFileFlag)
	if err != nil {
		fmt.Printf("error: failed to load certificates: %v\n", err)
		os.Exit(1)
	}
	dh := gemini.NewDomainHandler(*domainFlag, cert, h)
	ctx := context.Background()
	domainToHandler := map[string]*gemini.DomainHandler{
		*domainFlag: dh,
	}
	server := gemini.NewServer(ctx, fmt.Sprintf(":%d", *portFlag), domainToHandler)
	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
