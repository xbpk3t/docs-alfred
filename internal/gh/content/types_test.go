package content

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopicMarshalJSONProducesValidJSON(t *testing.T) {
	topic := Topic{
		Topic: "Parent",
		Qs:    []string{"question1"},
	}

	data, err := json.Marshal(&topic)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"topic":"Parent",
		"qs":["question1"]
	}`, string(data))
}
