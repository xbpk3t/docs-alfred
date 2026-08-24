package ghdata

import (
	"github.com/go-viper/mapstructure/v2"
	"github.com/xbpk3t/docs-alfred/internal/gh/model/gh"
)

// Repo/Topic are aliases of the schema-generated data model, so the walker
// decodes data/gh into the same types the schema validates. There is no
// section: the flat layout has one section per file, with its type tag derived
// from the file name (see WalkerEvent.SectionType).
type Repo = gh.Repo
type Topic = gh.Topic

func topicFromMap(m map[string]any) Topic {
	var topic Topic
	decodeYAMLMap(m, &topic)

	return topic
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
