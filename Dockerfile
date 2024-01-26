FROM golang:1.21 as build

COPY go.mod go.sum /
RUN go mod download

COPY main.go /
RUN CGO_ENABLED=0 go build -o /bin/app /main.go

FROM gcr.io/distroless/base
COPY --from=build /bin/app /app

ENV DEVICE_PATH=$DEVICE_PATH
ENV TELEGRAM_TOKEN=$TELEGRAM_TOKEN
CMD ["/app"]

