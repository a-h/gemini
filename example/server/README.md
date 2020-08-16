# Example Gemini server

## Generate private key (.key)

```sh
openssl ecparam -genkey -name secp384r1 -out server.key
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
```

Taken from example at https://github.com/denji/golang-tls

## Run server

```sh
go run main.go
```

## Use amfora or other browser to view

```sh
amfora gemini://localhost
```
