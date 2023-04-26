FROM alpine:3.16
RUN apk upgrade --no-cache

COPY ./build/linux /docker-credential-acr-env
ENV PATH "$PATH:/docker-credential-acr-env"
