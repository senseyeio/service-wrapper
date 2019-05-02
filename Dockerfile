ARG src=/go/src/github.com/senseyeio/service-wrapper
ARG consul=https://releases.hashicorp.com/envconsul/0.7.3/envconsul_0.7.3_linux_amd64.zip

FROM golang:1.10.2-alpine3.7 as src

ARG src
ARG consul

RUN apk --no-cache add upx

RUN wget -O /tmp/envconsul.zip ${consul} && unzip -d /bin /tmp/envconsul.zip

COPY . ${src}
WORKDIR ${src}

RUN ./build.sh

FROM scratch as run

ARG src

COPY --from=src ${src}/service-wrapper /bin/service-wrapper
COPY --from=src /bin/envconsul /bin/envconsul

