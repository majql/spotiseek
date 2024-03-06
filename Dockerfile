FROM alpine:latest
WORKDIR /app
COPY ./spotiseek.* /app/
ENTRYPOINT /app/spotiseek
