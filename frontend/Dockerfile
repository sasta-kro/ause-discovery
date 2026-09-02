FROM node:24.20.0-alpine3.23@sha256:0388af2af070cd4736a1567cfed02469ba117848845b4165d87a333edb53d2ca AS build

WORKDIR /src

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc ./
COPY api/generator/package.json api/generator/package.json
COPY frontend/package.json frontend/package.json

RUN pnpm install --frozen-lockfile

COPY frontend frontend

ARG VITE_PUBLIC_BASE_PATH=/ause-discovery/
ENV VITE_PUBLIC_BASE_PATH=${VITE_PUBLIC_BASE_PATH}

RUN pnpm --filter @ause-discovery/frontend build

FROM nginx:1.30.4-alpine@sha256:97d490c12ba55b4946b01546d1c3ed324e8d41ab1c9fcb2a616aa470620e5b46

ARG VITE_PUBLIC_BASE_PATH=/ause-discovery/

RUN public_base_path="${VITE_PUBLIC_BASE_PATH%/}" && \
    destination="/usr/share/nginx/html${public_base_path}" && \
    mkdir -p "${destination}"

COPY --from=build /src/frontend/dist/ /tmp/dist/

RUN public_base_path="${VITE_PUBLIC_BASE_PATH%/}" && \
    destination="/usr/share/nginx/html${public_base_path}" && \
    cp -a /tmp/dist/. "${destination}/" && \
    rm -rf /tmp/dist

COPY deploy/nginx/default.conf.template /etc/nginx/templates/default.conf.template

COPY deploy/nginx/20-base-path.envsh /docker-entrypoint.d/20-base-path.envsh

RUN chmod 755 /docker-entrypoint.d/20-base-path.envsh
