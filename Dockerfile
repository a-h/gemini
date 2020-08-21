FROM alpine:latest  

RUN apk --no-cache add ca-certificates

COPY gemini /gemini
COPY serve-gemini.sh /

RUN  chmod +x ./serve-gemini.sh

ENTRYPOINT ["./serve-gemini.sh"]
