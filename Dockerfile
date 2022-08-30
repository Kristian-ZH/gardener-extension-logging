# SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

############# builder
FROM golang:1.18.3 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-logging
COPY . .
RUN make install

############# gardener-extension-logging
FROM  gcr.io/distroless/static-debian11:nonroot AS gardener-extension-logging
WORKDIR /

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-logging /gardener-extension-logging
ENTRYPOINT ["/gardener-extension-logging"]
