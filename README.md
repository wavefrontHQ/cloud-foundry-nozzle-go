# run.sh
```
#!/usr/bin/env bash

export NOZZLE_API_URL=https://api.<domain>
export NOZZLE_USERNAME=<usr>
export NOZZLE_PASSWORD=<pass>
export NOZZLE_FIREHOSE_SUBSCRIPTION_ID=firehose-subscription-id
export NOZZLE_SKIP_SSL=true
export NOZZLE_SELECTED_EVENTS=ValueMetric,CounterEvent,ContainerMetric

export WAVEFRONT_URL=https://<instance>.wavefront.com
export WAVEFRONT_API_TOKEN=<api_token>
export WAVEFRONT_FLUSH_INTERVAL=15
export WAVEFRONT_PREFIX=pcf

go run main.go
```

# manifest.yml
```
---
applications:
  - name: wavefront-pcf-nozzle
    command: pcf-nozzle
    no-route: true
    health-check-type: process
    buildpack: https://github.com/cloudfoundry/go-buildpack.git
    env:
      NOZZLE_API_URL: https://api.......
      NOZZLE_USERNAME: admin
      NOZZLE_PASSWORD: ......

      NOZZLE_FIREHOSE_SUBSCRIPTION_ID: firehose-subscription-id
      NOZZLE_SKIP_SSL: true
      NOZZLE_SELECTED_EVENTS: ValueMetric,CounterEvent,ContainerMetric

      WAVEFRONT_URL: https://......wavefront.com
      WAVEFRONT_API_TOKEN: .........
      WAVEFRONT_FLUSH_INTERVAL: 15
      WAVEFRONT_PREFIX: pcf
```

```
cf push wavefront-pcf-nozzle
```
