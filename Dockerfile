FROM golang:1.10

# Get git
RUN apt-get update \
    && apt-get -y install curl git \
    && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Get Dep to install ethereum-go
RUN  curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

# Where anchormaker sources will live
WORKDIR $GOPATH/src/github.com/FactomProject/anchormaker-eth

# Populate the rest of the source
COPY . .
RUN dep ensure

#RUN cp -r \
#"${GOPATH}/src/github.com/ethereum/go-ethereum/crypto/secp256k1/libsecp256k1" \
#"vendor/github.com/ethereum/go-ethereum/crypto/secp256k1/"

ARG GOOS=linux

# Build and install anchormaker
RUN ./build.sh

ENTRYPOINT ["/go/bin/anchormaker","-sim_stdin=false"]
