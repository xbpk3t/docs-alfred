package ghdata

import (
	"github.com/go-viper/mapstructure/v2"
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
)

// Section/Repo/Topic are aliases of the schema-generated data model, so the
// walker decodes data/gh into the same types the schema validates.
type Section = gh.Section
type Repo = gh.Repo
type Topic = gh.Topic

func sectionFromMap(m map[string]any) Section {
	var section Section
	decodeYAMLMap(m, &section)

	return section
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
