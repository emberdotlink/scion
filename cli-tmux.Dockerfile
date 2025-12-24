# Assumes a local copy of gemini-cli-sandbox


FROM gemini-cli-sandbox
USER root

RUN apt-get update && apt-get install -y --no-install-recommends \
  tmux \
  curl \
  git \
  wget \
  make libssl-dev libcurl4-gnutls-dev libexpat1-dev libghc-zlib-dev gettext \
  ca-certificates \
  && apt-get clean \
  && rm -rf /var/lib/apt/lists/*

# Install Go
ARG GO_VERSION=1.25.4
RUN ARCH=$(dpkg --print-architecture) && \
    case "${ARCH}" in \
      amd64) GO_ARCH='linux-amd64' ;; \
      arm64) GO_ARCH='linux-arm64' ;; \
      *) echo "Unsupported architecture: ${ARCH}"; exit 1 ;; \
    esac && \
    curl -L "https://go.dev/dl/go${GO_VERSION}.${GO_ARCH}.tar.gz" -o go.tar.gz && \
    tar -C /usr/local -xzf go.tar.gz && \
    rm go.tar.gz

  # Install git from source
  # later version required to support git worktree relative paths
WORKDIR /opt

RUN wget https://www.kernel.org/pub/software/scm/git/git-2.52.0.tar.gz
RUN tar -xvf git-2.52.0.tar.gz
WORKDIR /opt/git-2.52.0/
RUN make prefix=/usr/local all
RUN make prefix=/usr/local install
RUN rm /usr/bin/git
RUN ln -s /usr/local/bin/git /usr/bin/git
RUN rm -r /opt/


ENV PATH=/usr/local/go/bin:$PATH

USER node