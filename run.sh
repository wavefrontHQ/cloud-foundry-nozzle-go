#!/usr/bin/env bash

export NOZZLE_API_URL=https://api.<domain>
export NOZZLE_CLIENT_ID=<client id>
export NOZZLE_CLIENT_SECRET=<client secret>
export NOZZLE_FIREHOSE_SUBSCRIPTION_ID=firehose-subscription-id
export NOZZLE_SKIP_SSL=true
export NOZZLE_SELECTED_EVENTS=ValueMetric,CounterEvent,ContainerMetric

export WAVEFRONT_URL=https://<instance>.wavefront.com
export WAVEFRONT_API_TOKEN=<api_token>
export WAVEFRONT_FLUSH_INTERVAL=15
export WAVEFRONT_PREFIX=pcf

go run main.go
