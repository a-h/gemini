build:
	go build -o gemini ./cmd/main.go

build-docker:
	docker build . -t adrianhesketh/gemini

build-snapshot:
	goreleaser build --snapshot --rm-dist

serve-local-tests:
	@echo add '127.0.0.1       a-h.gemini' to your /etc/hosts file
	openssl ecparam -genkey -name secp384r1 -out server.key
	openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650 -subj "/C=/ST=/L=/O=/OU=/CN=a-h.gemini"
	go run ./cmd/main.go serve --domain=a-h.gemini --certFile=server.crt --keyFile=server.key --path=./tests

release: 
	if [ "${GITHUB_TOKEN}" == "" ]; then echo "Set the GITHUB_TOKEN environment variable"; fi
	./push-tag.sh
	goreleaser --rm-dist
