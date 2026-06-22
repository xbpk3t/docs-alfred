package content

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicMarshalJSONProducesValidJSON(t *testing.T) {
	topic := Topic{
		Topic:  "Parent",
		Des:    "description",
		Meta:   &TopicMeta{Slug: "parent", HasPic: true},
		HasPic: true,
		Sub: Topics{
			{Topic: "Child"},
		},
	}

	data, err := json.Marshal(&topic)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"topic":"Parent",
		"des":"description",
		"sub":[{"topic":"Child"}],
		"hasPic":true
	}`, string(data))
	assert.NotContains(t, string(data), "meta")
}
