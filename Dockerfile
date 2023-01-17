FROM golang:1.18.1 as builder
ADD . /build
WORKDIR /build
RUN go vet ./...
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o build/openvpnas_exporter

FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /build/build/openvpnas_exporter /bin/openvpnas_exporter
EXPOSE 9176
ENV GOMEMLIMIT=256MiB
ENTRYPOINT ["/bin/openvpnas_exporter"]
