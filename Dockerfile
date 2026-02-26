FROM scratch
ENV HOME=/root
COPY hawkeye /hawkeye
ENTRYPOINT ["/hawkeye"]
