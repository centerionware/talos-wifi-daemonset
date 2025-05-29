FROM Alpine:3.21.3
# This was the latest at time of writing
RUN apk add --no-cache wpa_supplicant kubectl
#   && apk del bash
copy entry_point.sh /
copy template.conf /
run chmod +x /entrypoint.sh
ENTRYPOINT /entry_point.sh

