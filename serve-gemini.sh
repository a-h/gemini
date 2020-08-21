#!/usr/bin/env sh
pid=0

# Respond to ctrl-c as per https://medium.com/@gchudnov/trapping-signals-in-docker-containers-7a57fdda7d86

# SIGTERM-handler
term_handler() {
  if [ $pid -ne 0 ]; then
    kill -SIGTERM "$pid"
    wait "$pid"
  fi
  exit 143; # 128 + 15 -- SIGTERM
}

# setup handlers
# on callback, kill the last background process, which is `tail -f /dev/null` and execute the specified handler
trap 'kill ${!}; term_handler' SIGTERM

# Configure defaults.
if [ "$PORT" = "" ];
then
	export PORT=1965;
fi
if [ "$DOMAIN" = "" ];
then
	export DOMAIN=localhost;
fi
if [ ! -d /certs ]; 
then
	echo "Docker usage:"
	echo ""
	echo "Create server certificates with the domain set as the common name (all other fields can be left default):"
	echo "  openssl ecparam -genkey -name secp384r1 -out server.key"
	echo "  openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650"
	echo ""
	echo "Then run Docker container:"
	echo ""
	echo "  docker run -v /path_to_your_cert_files:/certs -e PORT=1965 -e DOMAIN=localhost -v /path_to_your_content:/content -p 1965:1965 adrianhesketh/gemini:latest"
	echo ""
	return
fi
if [ ! -e /certs/server.crt ];
then
	echo "server.crt file not found, are your certs named correctly?"
fi
if [ ! -e /certs/server.key ];
then
	echo "server.key file not found, are your certs named correctly?"
fi

# run application
# Run server.
./gemini serve --path=/content --certFile=/certs/server.crt --keyFile=/certs/server.key --port=$PORT --domain=$DOMAIN &
pid="$!"

# wait forever
while true
do
  tail -f /dev/null & wait ${!}
done
