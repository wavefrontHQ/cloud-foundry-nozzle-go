# Wavefront cloud-foundry-nozzle [![build status][ci-img]][ci] [![Go Report Card][go-report-img]][go-report]
Wavefront Go Nozzle for Pivotal Cloud Foundry (PCF). Included as part of the [Wavefront by VMware Nozzle for PCF](https://network.pivotal.io/products/wavefront-nozzle/).

## Manually configure and deploy to PCF

1. `git clone github.com/wavefrontHQ/cloud-foundry-nozzle-go`
2. Update the required properties in `manifest.yml`
3. Run `cf push` to deploy the nozzle to PCF

## Run Locally
1. Edit the required properties in `run.sh`
2. Run `run.sh` to manually run the Nozzle.

## Contributing
Public contributions are welcome. Please feel free to report issues or submit pull requests.

[ci-img]: https://travis-ci.com/wavefrontHQ/cloud-foundry-nozzle-go.svg?branch=master
[ci]: https://travis-ci.com/wavefrontHQ/cloud-foundry-nozzle-go
[go-report-img]: https://goreportcard.com/badge/github.com/wavefronthq/cloud-foundry-nozzle-go
[go-report]: https://goreportcard.com/report/github.com/wavefronthq/cloud-foundry-nozzle-go
