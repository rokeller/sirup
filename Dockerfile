# CA certs - we need a decent set of CA certificates so outgoing TLS channels
# can successfully be established on the below image from scratch.
FROM alpine AS certs

# Build the final image
FROM scratch

USER 1000:1000
WORKDIR /app
ENTRYPOINT [ "/app/sirup" ]
EXPOSE 8080/tcp

COPY --link --from=alpine /etc/ssl/certs /etc/ssl/certs
COPY --link --chown=1000:1000 sirup ./
