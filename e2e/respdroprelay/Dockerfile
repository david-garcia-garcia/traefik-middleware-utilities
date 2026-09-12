FROM golang:1.21-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /respdroprelay .

FROM alpine:3.20
COPY --from=build /respdroprelay /respdroprelay
EXPOSE 6379
ENTRYPOINT ["/respdroprelay"]
