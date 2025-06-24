FROM ubuntu:22.04

LABEL org.opencontainers.image.source="https://github.com/autocrawlerHQ/browsergrid"

ENV DEBIAN_FRONTEND=noninteractive

ENV TZ=America/Los_Angeles

ENV HOME=/home/user
ENV LANG=en_US.UTF-8
ENV LC_ALL=en_US.UTF-8

# SCREEN
ENV DISPLAY=:1
ENV VNC_PORT=5900
ENV VNC_PASSWORD=password
ENV RESOLUTION_WIDTH=1920
ENV RESOLUTION_HEIGHT=1080
ENV RESOLUTION_DEPTH=24

ENV RESOLUTION=${RESOLUTION_WIDTH}x${RESOLUTION_HEIGHT}x${RESOLUTION_DEPTH}

# Set static UID and GID for the browser user
ENV UID=1000
ENV GID=1000

ENV REMOTE_DEBUGGING_PORT=61000
ENV SW_SERVER_PORT=8082

# TODO: USER DATA DIR and extensions should be custamizable


RUN apt-get update && apt-get install -y --no-install-recommends \
    supervisor \
    xvfb \
    x11vnc \
    novnc \
    xterm \
    dbus-x11 \
    net-tools \
    software-properties-common \
    websockify \
    locales \
    tzdata \
    ca-certificates \
    wget \
    curl \
    supervisor \
    pwgen \
    socat \
    tar \
    unzip \
    gnupg \
    xdg-utils \
    x11-utils \
    libnss3 \
    libatk1.0-0 \
    libatk-bridge2.0-0 \
    libcups2 \
    libdrm2 \
    libxkbcommon0 \
    libxcomposite1 \
    libxdamage1 \
    libxfixes3 \
    libxrandr2 \
    libgbm1 \
    libasound2 \
    nginx \
    && apt-get autoclean \
	&& apt-get autoremove \
    && rm -rf /var/lib/apt/lists/*

RUN apt-get update && apt-get -y --no-install-recommends install \
    fontconfig \
    fonts-freefont-ttf \
    fonts-gfs-neohellenic \
    fonts-indic \
    fonts-ipafont-gothic \
    fonts-kacst \
    fonts-liberation \
    fonts-noto-cjk \
    fonts-noto-color-emoji \
    fonts-roboto \
    fonts-thai-tlwg \
    fonts-ubuntu \
    fonts-wqy-zenhei \
    fonts-open-sans \
    ttf-mscorefonts-installer \
    cabextract \
    && fc-cache -f -v

# Install Node.js 22
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

RUN localedef -i en_US -c -f UTF-8 -A /usr/share/locale/locale.alias en_US.UTF-8

RUN ln -fs /usr/share/zoneinfo/${TZ} /etc/localtime && \
    dpkg-reconfigure -f noninteractive tzdata



# Create startup scripts
RUN mkdir -p /opt/browsergrid/scripts

# Create non-root user with static UID/GID
RUN groupadd -g ${GID} browseruser && \
    useradd -u ${UID} -g ${GID} -s /bin/bash -m -d ${HOME} browseruser

# Copy scripts first, then set permissions
COPY docker/scripts/ /opt/browsergrid/scripts/
RUN chmod -R +x /opt/browsergrid/scripts/ && \
    chown -R browseruser:browseruser /opt/browsergrid/scripts

# Set proper permissions for directories that need to be accessed by the non-root user
RUN mkdir -p ${HOME}/.cache && \
    mkdir -p ${HOME}/.config && \
    mkdir -p ${HOME}/.npm && \
    chown -R browseruser:browseruser /opt/browsergrid && \
    chown -R browseruser:browseruser ${HOME} && \
    chmod -R 755 ${HOME}





# supervisor
COPY docker/browsers/base/supervisord.conf /etc/supervisor/conf.d/supervisord.conf


# entrypoint.sh
COPY docker/browsers/base/entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh

# Set up nginx configuration
RUN mkdir -p /etc/nginx/conf.d
COPY docker/browsers/base/nginx/nginx.conf /etc/nginx/conf.d/default.conf

# Add nginx setup script
COPY docker/browsers/base/nginx/setup_nginx.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/setup_nginx.sh

# We will keep running as root for the entrypoint
# because Xvfb and x11vnc need root permissions
# Individual browser processes will be run as browseruser
WORKDIR ${HOME}

EXPOSE 5900
EXPOSE 6080
EXPOSE 9222
EXPOSE 8765

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

# keep container running
CMD ["tail", "-f", "/dev/null"]