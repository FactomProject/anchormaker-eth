FROM golang:1.11

WORKDIR /go/bin/
COPY anchormaker-eth .
RUN chmod +x anchormaker-eth

RUN mkdir -p /root/.factom/m2
COPY anchormaker.conf /root/.factom/anchormaker.conf

ENTRYPOINT ["./anchormaker-eth"]
