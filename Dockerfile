FROM golang:1.26.4-bookworm AS build

WORKDIR /src
COPY go.mod ./
COPY go.sum ./
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/pawit-api .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/pawit-migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/pawit-api /app/pawit-api
COPY --from=build /out/pawit-migrate /app/pawit-migrate

ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/app/pawit-api"]
