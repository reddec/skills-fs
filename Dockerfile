# goreleaser-managed image: it stages the built binary per platform and this Dockerfile
# selects the right one via TARGETPLATFORM.
FROM --platform=$BUILDPLATFORM alpine:3.20 AS certs
RUN apk add --no-cache ca-certificates && update-ca-certificates

FROM scratch
ARG TARGETPLATFORM
WORKDIR /data
VOLUME ["/data"]
EXPOSE 8080
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY $TARGETPLATFORM/skills-fs /bin/skills-fs
ENTRYPOINT ["/bin/skills-fs"]
