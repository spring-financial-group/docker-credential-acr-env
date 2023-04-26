FROM alpine:3.16
RUN apk upgrade --no-cache

COPY ./build/docker-credential-acr-env /docker-credential-acr-env
ENV PATH "$PATH:/docker-credential-acr-env"
