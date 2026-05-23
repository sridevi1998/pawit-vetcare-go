FROM golang:1.22.1-bookworm AS build

WORKDIR /src
COPY go.mod ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/pawit-api .

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/pawit-api /app/pawit-api

ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/pawit-api"]
