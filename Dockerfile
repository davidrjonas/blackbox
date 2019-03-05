FROM golang:1.12-alpine as build

RUN apk add --no-cache git

WORKDIR /src
COPY . .

RUN CGO_ENABLED=0 go test -c -o blackbox

FROM alpine:3.9

COPY --from=build /src/blackbox /usr/local/bin/

RUN apk add -U --no-cache ca-certificates

WORKDIR /tests

ENTRYPOINT ["blackbox"]
