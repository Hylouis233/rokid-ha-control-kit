FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
COPY . ./
RUN CGO_ENABLED=0 go build -o /out/rokid-ha-control-kit .

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/rokid-ha-control-kit /app/rokid-ha-control-kit
COPY config /app/config
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/rokid-ha-control-kit"]
