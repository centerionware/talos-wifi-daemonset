FROM alpine:3.21.3
# This was the latest at time of writing
RUN apk add --no-cache wpa_supplicant kubectl
#   && apk del bash
COPY entry_point.sh /
RUN chmod +x /entry_point.sh
ENTRYPOINT ["/entry_point.sh"]

