FROM fedora:27

COPY cmd/mesitis/mesitis /opt/services/mesitis

ENTRYPOINT ["/opt/services/mesitis"]
