FROM golang:1.20-alpine3.18 AS dex-builder
RUN apk add make
WORKDIR /makeen-dex
ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN make build

FROM alpine:latest
WORKDIR /makeen-dex
COPY --from=dex-builder /makeen-dex/makeen-dex ./
COPY ./openapi ./openapi
# ADD ./msp ./msp/
RUN ln -s /makeen-dex/makeen-dex /usr/bin/makeen-dex
ENTRYPOINT [ "makeen-dex" ]