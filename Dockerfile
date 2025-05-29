FROM go as build
COPY entrypoint.go /
RUN go build entrypoint.go


FROM alpine:3.21.3
# This was the latest at time of writing
RUN apk add --no-cache wpa_supplicant kubectl && apk cache purge && apk del apk-tools
#   && apk del bash
COPY --from=build entrypoint /

ENTRYPOINT ["/entrypoint"]

