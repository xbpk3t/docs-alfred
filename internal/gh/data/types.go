package ghdata

import (
	"github.com/go-viper/mapstructure/v2"
	"github.com/xbpk3t/docs-alfred/internal/gh/model"
)

// Section/Repo/Topic are aliases of the schema-generated data model, so the
// walker decodes data/gh into the same types the schema validates.
type Section = model.Section
type Repo = model.Repo
type Topic = model.Topic

func sectionFromMap(m map[string]any) Section {
	var section Section
	decodeYAMLMap(m, &section)

	return section
}

func repoFromMap(m map[string]any) Repo {
	var repo Repo
	decodeYAMLMap(m, &repo)

	return repo
}

func decodeYAMLMap(input, output any) {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:  output,
		TagName: "yaml",
	})
	if err != nil {
		return
	}
	_ = decoder.Decode(input)
}
