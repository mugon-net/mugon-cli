# syntax=docker/dockerfile:1
# CI image for the mugon CLI. Variants (build with --target):
#   cli   mugon only (linux/amd64, linux/arm64)
#   slim  cli + mugon-editor: import, export, MCP, serve; no GUI (linux/amd64)
#   full  slim + mugon-editor-app / mugon-editor-thumbs and Vulkan software drivers (linux/amd64)
# Expects the released binaries in docker-bin/<os>/<arch>/mugon (the release workflow unpacks them from the archives).
ARG DEBIAN_RELEASE=bookworm-slim

FROM debian:${DEBIAN_RELEASE} AS base
ARG TARGETOS
ARG TARGETARCH
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
COPY docker-bin/${TARGETOS}/${TARGETARCH}/mugon /usr/local/bin/mugon
RUN useradd --create-home --uid 1000 mugon
ENV HOME=/home/mugon
WORKDIR /workspace
USER mugon

FROM base AS cli

FROM base AS editor-install
ARG MUGON_TOOLS_URL
ENV MUGON_TOOLS_URL=${MUGON_TOOLS_URL}
RUN mugon editor install

FROM base AS slim
COPY --chown=mugon:mugon --from=editor-install /home/mugon/.mugon/editor /home/mugon/.mugon/editor
USER root
RUN version="$(cat /home/mugon/.mugon/editor/current)" \
    && rm -f "/home/mugon/.mugon/editor/${version}/mugon-editor-app" "/home/mugon/.mugon/editor/${version}/mugon-editor-thumbs" \
    && ln -s "/home/mugon/.mugon/editor/${version}/mugon-editor" /usr/local/bin/mugon-editor
USER mugon

FROM base AS full
# Package list for headless rendering is a first guess; verify with a thumbnail run (see plan, phase 6).
USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends libvulkan1 mesa-vulkan-drivers libgl1 libx11-6 libxkbcommon0 libwayland-client0 libudev1 \
    && rm -rf /var/lib/apt/lists/*
COPY --chown=mugon:mugon --from=editor-install /home/mugon/.mugon/editor /home/mugon/.mugon/editor
RUN version="$(cat /home/mugon/.mugon/editor/current)" \
    && for executable in mugon-editor mugon-editor-app mugon-editor-thumbs; do \
         ln -s "/home/mugon/.mugon/editor/${version}/${executable}" "/usr/local/bin/${executable}"; \
       done
USER mugon
