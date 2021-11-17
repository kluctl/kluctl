FROM python:3.10.0-slim-buster

RUN apt update && apt install wget libyaml-dev git -y && rm -rf /var/lib/apt/lists/*

# Install tools
ENV KUSTOMIZE_VERSION=v4.4.1
ENV HELM_VERSION=v3.7.0
ENV KUBESEAL_VERSION=v0.16.0
RUN wget -O kustomize.tar.gz https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz && \
    tar xzf kustomize.tar.gz && \
    mv kustomize /usr/bin && \
    rm kustomize.tar.gz
RUN wget -O helm.tar.gz https://get.helm.sh/helm-$HELM_VERSION-linux-amd64.tar.gz && \
    tar xzf helm.tar.gz && \
    mv linux-amd64/helm /usr/bin && \
    rm helm.tar.gz
RUN wget -O kubeseal https://github.com/bitnami-labs/sealed-secrets/releases/download/$KUBESEAL_VERSION/kubeseal-linux-amd64 && \
    chmod +x kubeseal && \
    mv kubeseal /usr/bin

RUN apt update && apt install gcc -y && \
    pip install "pyyaml==5.4.1" --global-option=--with-libyaml && \
    apt remove gcc -y && apt autoremove -y && rm -rf /var/lib/apt/lists/*

ADD . /kluctl
RUN pip install --no-cache-dir /kluctl

ENTRYPOINT /usr/local/bin/kluctl
