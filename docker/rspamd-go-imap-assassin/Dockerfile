FROM a16bitsysop/rspamd

LABEL maintainer="crawx <crux@project-insanity.org>"

# copy go-spam-assassin tailored configs
WORKDIR /etc/rspamd/local.d
COPY local.d ./

# reset workdir
WORKDIR /usr/local/bin
# decrease conservative redis sleep from upstream image
RUN sed -i 's/sleep 60s/sleep 10s/g' entrypoint.sh

# enable redis, dccifd, bazaar and heinlein, disable greylisting
ENV REDIS=redis DCCIFD=dccifd NOGREY=true BZSLEEP=1 HLSLEEP=1

CMD [ "entrypoint.sh" ]
