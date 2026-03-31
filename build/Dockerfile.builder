FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    make \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

ENV GOCACHE=/workspace/.cache/go-build
ENV GOPATH=/workspace/.cache/gopath
ENV GOMODCACHE=/workspace/.cache/gomod
ENV NPM_CONFIG_CACHE=/workspace/.cache/npm

CMD ["make", "dist-linux-amd64"]
