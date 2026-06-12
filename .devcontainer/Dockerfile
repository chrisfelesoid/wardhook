# syntax=docker/dockerfile:1
FROM debian:bookworm AS base

ARG APPDIR=/workspace
ARG CUSTOM_USERNAME=appuser
ARG USER_UID=1000
ARG USER_GID=$USER_UID
ARG HOME="/home/$CUSTOM_USERNAME"
ENV PATH="$HOME/.local/bin:$PATH"
WORKDIR $APPDIR

RUN rm -f /etc/apt/apt.conf.d/docker-clean; \
    echo 'Binary::apt::APT::Keep-Downloaded-Packages "true";' > /etc/apt/apt.conf.d/keep-cache

# create user
RUN useradd -s /bin/bash -u $USER_UID -o -c "" -m $CUSTOM_USERNAME && \
    groupmod -g $USER_UID $CUSTOM_USERNAME

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends sudo \
    && echo $CUSTOM_USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$CUSTOM_USERNAME \
    && chmod 0440 /etc/sudoers.d/$CUSTOM_USERNAME

# base
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y --no-install-recommends openssh-client ca-certificates git git-lfs gh curl unzip zip build-essential

USER $CUSTOM_USERNAME
RUN curl https://mise.run | sh
RUN echo 'eval "$(~/.local/bin/mise activate bash)"' >> ~/.bashrc

FROM base AS dev
