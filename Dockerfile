FROM alpine:latest  

RUN apk --no-cache add ca-certificates

COPY dist/gemini_linux_amd64/gemini /gemini
COPY serve-gemini.sh /

RUN  chmod +x ./serve-gemini.sh

ENTRYPOINT ["./serve-gemini.sh"]
