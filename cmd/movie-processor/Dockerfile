FROM golang:1.18 as build-env

ENV GOPROXY=direct
WORKDIR /go/src/app

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -o /go/bin/app cmd/movie-processor/*.go

FROM gcr.io/distroless/static

COPY --from=build-env /go/bin/app /
EXPOSE 8080
CMD ["/app"]