# Treatment creation for v2 product page experiments

The treatment creation command previously always sent the v1 experiment relationship, even for an experiment created with `experiments create --v2`. The published `AppStoreVersionExperimentTreatmentCreateRequest` schema supports both relationships at the same `POST /v1/appStoreVersionExperimentTreatments` endpoint.

Add `asc product-pages experiments treatments create --experiment-id ID --name NAME --v2`. The flag selects `appStoreVersionExperimentV2`; the default continues to select `appStoreVersionExperiment`. Both carry resource type `appStoreVersionExperiments`. Exactly one relationship is serialized. The existing client method retains its v1 behavior and a separate V2 method selects the new relationship through a shared private implementation.

Required flags, output formats, mutation response rendering, timeout, and error handling stay consistent with the existing command. No migration is required. Automatic version detection would add an extra request and ambiguous failure handling; explicit version selection matches existing experiment commands.

The CLI regression test covers both request bodies and the unchanged POST route. Before the fix, its v2 case fails because `--v2` is unknown. Live verification uses a temporary, unstarted draft experiment on the authorized test app, with cleanup performed by the coordinating agent.
