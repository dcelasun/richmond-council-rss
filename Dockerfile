FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /richmond-council-rss .

FROM gcr.io/distroless/static-debian12
COPY --from=build /richmond-council-rss /richmond-council-rss
EXPOSE 8080
ENTRYPOINT ["/richmond-council-rss"]
