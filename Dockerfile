FROM golang:1.26-alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" -o /out/rit-cal-sync ./cmd/rit-cal-sync

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
COPY --from=build /out/rit-cal-sync /usr/local/bin/rit-cal-sync
USER app
WORKDIR /app
ENV CALENDAR_URL=https://www.rit.edu/calendar TERM_SEASONS=Fall,Spring TIMEZONE=America/New_York
VOLUME ["/secrets"]
ENTRYPOINT ["/usr/local/bin/rit-cal-sync"]
