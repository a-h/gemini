package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/a-h/gemini"
)

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
	}
	usage()
}

func usage() {
	fmt.Println(`usage: gemini <command> [parameters]
To see help text, you can run:

  gemini request --help
  gemini serve --help

examples:

  gemini request --insecure --verbose gemini://example.com/pass
  gemini serve --host=example.com --certFile=a.crt --keyFile=a.key --path=.`)
	os.Exit(1)
}

func request(args []string) {
	cmd := flag.NewFlagSet("request", flag.ExitOnError)
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
		client.AddCertificateForURLPrefix("/", keyPair)
	}
	resp, certificates, authenticated, ok, err := client.RequestURL(u)
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
		io.Copy(os.Stdout, resp.Body)
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
	dh, err := gemini.NewDomainHandler(*domainFlag, *certFileFlag, *keyFileFlag, h)
	if err != nil {
		fmt.Printf("error: failed to create handler: %v\n", err)
		os.Exit(1)
	}
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
