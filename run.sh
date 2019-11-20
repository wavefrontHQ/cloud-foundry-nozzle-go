#!/usr/bin/env bash

export NOZZLE_API_URL=https://uaa.sys.patterson.cf-app.com
export NOZZLE_USERNAME=glp-v2-nozzle
export NOZZLE_PASSWORD=seosal99

export NOZZLE_LOG_STREAM_URL="https://log-stream.sys.patterson.cf-app.com"

export NOZZLE_SELECTED_EVENTS=log,counter,gauge,timer,event
export NOZZLE_SKIP_SSL=true
export NOZZLE_FIREHOSE_SUBSCRIPTION_ID=local-firehose-subscription-id

export WAVEFRONT_URL=https://nimba.wavefront.com
export WAVEFRONT_API_TOKEN=33a0e409-8851-4574-b5c8-09833d90457c
export WAVEFRONT_FLUSH_INTERVAL=15
export WAVEFRONT_PREFIX=pcf
export WAVEFRONT_FOUNDATION=command_line

export WAVEFRONT_DEBUG=false

go run main.go
