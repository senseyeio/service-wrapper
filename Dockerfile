ARG src=/go/src/github.com/senseyeio/service-wrapper
ARG consul=https://releases.hashicorp.com/envconsul/0.9.1/envconsul_0.9.1_linux_amd64.zip

FROM golang:1.13.4 as src

ARG src
ARG consul

RUN apt-get update && apt-get install -y upx-ucl unzip

RUN wget -O /tmp/envconsul.zip ${consul} && unzip -d /bin /tmp/envconsul.zip

COPY . ${src}
WORKDIR ${src}

RUN ./build.sh

FROM scratch as run

ARG src

COPY --from=src /bin/envconsul /bin/envconsul
COPY --from=src ${src}/service-wrapper /bin/service-wrapper

