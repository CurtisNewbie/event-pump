package pump

import "github.com/curtisnewbie/miso/miso"

// misoapi-http: POST /api/v1/create-pipeline
// misoapi-desc: Create new pipeline. Duplicate pipeline is ignored, HA is not supported.
func ApiCreatePipeline(rail miso.Rail, pipeline ApiPipeline) error {
	p := pipeline.Pipeline()
	if isHaMode() {
		return miso.NewErrf("Not supported for HA mode")
	}
	return AddPipeline(rail, p)
}

// misoapi-http: POST /api/v1/remove-pipeline
// misoapi-desc: Remove existing pipeline. HA is not supported.
func ApiRemovePipeline(rail miso.Rail, pipeline ApiPipeline) error {
	p := pipeline.Pipeline()
	if isHaMode() {
		return miso.NewErrf("Not supported for HA mode")
	}
	RemovePipeline(rail, p)
	return nil
}

// misoapi-http: GET /api/v1/list-pipeline
// misoapi-desc: List existing pipeline. HA is not supported.
func ApiListPipelines(rail miso.Rail) ([]ApiPipeline, error) {
	if isHaMode() {
		return nil, miso.NewErrf("Not supported for HA mode")
	}
	p := copyApiPipelines()
	return p, nil
}
