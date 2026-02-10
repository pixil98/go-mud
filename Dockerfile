# Build stage
FROM golang:1.24-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /app/mud ./cmd/mud

# Runtime stage
FROM alpine:3.21

WORKDIR /app

COPY --from=build /app/mud .
COPY config.json .
COPY assets/commands/ assets/commands/
COPY assets/mobiles/ assets/mobiles/
COPY assets/objects/ assets/objects/
COPY assets/pronouns/ assets/pronouns/
COPY assets/races/ assets/races/
COPY assets/rooms/ assets/rooms/
COPY assets/zones/ assets/zones/

RUN mkdir -p assets/characters

EXPOSE 4000

ENTRYPOINT ["/app/mud"]
