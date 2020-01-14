package utils_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wavefronthq/cloud-foundry-nozzle-go/internal/utils"
)

func TestGetVcapApp(t *testing.T) {
	app, err := utils.GetVcapApp()
	if err == nil {
		assert.FailNow(t, "VCAP_APPLICATION")
	}

	os.Setenv("VCAP_APPLICATION", "{\"application_id\":\"d6c8081c-4ee7-4f23-91f1-1d4414e2c24d\",\"application_name\":\"wavefront-firehose-nozzle-1.0.0\",\"application_uris\":[],\"application_version\":\"836082c5-da44-4985-beec-886cd28c0d37\",\"cf_api\":\"https://api.sys.dev.wavefront.io\",\"host\":\"0.0.0.0\",\"instance_id\":\"3b87cd48-3616-4458-4184-6def\",\"instance_index\":0,\"limits\":{\"disk\":1024,\"fds\":16384,\"mem\":2048},\"name\":\"wavefront-firehose-nozzle-1.0.0\",\"port\":8080,\"space_id\":\"07da7bc4-318f-428f-a97e-606179a30d7e\",\"space_name\":\"wavefront-apps-space\",\"uris\":[],\"version\":\"836082c5-da44-4985-beec-886cd28c0d37\"}")

	app, err = utils.GetVcapApp()
	if err != nil {
		assert.FailNow(t, "VCAP_APPLICATION: ", err)
	}

	assert.Equal(t, "wavefront-firehose-nozzle-1.0.0", app.Name, "VCAP_APPLICATION")

}
