FROM browsergrid/base:latest

LABEL org.opencontainers.image.description="WebKit browser for Browsergrid"
LABEL org.opencontainers.image.version="${BROWSER_VERSION}"
LABEL browsergrid.browser.version="${BROWSER_VERSION}"
LABEL browsergrid.playwright.version="${PLAYWRIGHT_VERSION}"

ARG PLAYWRIGHT_VERSION=latest
ARG BROWSER_VERSION=latest

RUN apt-get update && apt-get install -y sudo && \
    echo "browseruser ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers && \
    rm -rf /var/lib/apt/lists/*

RUN npm i playwright@${PLAYWRIGHT_VERSION}

RUN npx playwright install --with-deps webkit

# Create WebKit data directory with proper permissions
RUN mkdir -p ${HOME}/webkit-data && \
    chown -R browseruser:browseruser ${HOME}/webkit-data && \
    chmod -R 755 ${HOME}/webkit-data

COPY docker/browsers/webkit/start.sh /start.sh
COPY docker/browsers/webkit/docker-healthcheck /usr/local/bin/docker-healthcheck
RUN chmod +x /start.sh /usr/local/bin/docker-healthcheck
RUN chown browseruser:browseruser /start.sh

HEALTHCHECK --interval=5s --timeout=3s --retries=3 CMD ["docker-healthcheck"]

# Run WebKit as browseruser while keeping the container's main process as root
CMD ["su", "-c", "/start.sh", "browseruser"]
