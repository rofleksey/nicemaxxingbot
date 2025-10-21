package whisper

import (
	"context"
	"nicemaxxingbot/app/config"
	"testing"

	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhisper(t *testing.T) {
	cfg, err := config.Load("../../../config.yaml")
	require.NoError(t, err)

	di := do.New()
	do.ProvideValue(di, cfg)

	client, err := NewClient(di)
	require.NoError(t, err)

	res, err := client.TranscribeFile(context.Background(), "test.mp3")
	require.NoError(t, err)

	assert.Equal(t, res, "*Evil laughter* I love this shit so much bro! Ohhhh... What do we call this? What do we call this? There has to be a name. We have to. We have to have a name for this shit.")
}
