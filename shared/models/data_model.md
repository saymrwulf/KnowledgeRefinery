# Canonical Data Model

See [../../docs/data-model.md](../../docs/data-model.md) for the full data model documentation.

## Pipeline Version Tracking

Every derived artifact stores:
- `pipeline_version` - Overall pipeline version (e.g., "v1.0")
- `model_id` - LM Studio model used (for annotations/concepts)
- `prompt_id` + `prompt_version` - Prompt template version (for annotations)

This enables:
- Re-running with updated models/prompts without losing previous results
- A/B comparison of different model outputs
- Audit trail of how insights were derived
