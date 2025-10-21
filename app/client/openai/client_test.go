package openai

import (
	"context"
	"nicemaxxingbot/app/config"
	"testing"
	"time"

	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToxicity_TableDriven(t *testing.T) {
	cfg, err := config.Load("../../../config.yaml")
	require.NoError(t, err)

	di := do.New()
	do.ProvideValue(di, cfg)

	client, err := NewClient(di)
	require.NoError(t, err)

	tests := []struct {
		phrase         string
		expectedToxic  bool
		expectedPhrase string
	}{
		{
			phrase:         "Blight players are not human",
			expectedToxic:  true,
			expectedPhrase: "Blight players are not human",
		},
		{
			phrase:         "Blights are absolutely dogshit at this game",
			expectedToxic:  true,
			expectedPhrase: "Blights are absolutely dogshit at this game",
		},
		{
			phrase:         "Hello, how are you?",
			expectedToxic:  false,
			expectedPhrase: "",
		},
		{
			phrase:         "Eat shit, loser",
			expectedToxic:  true,
			expectedPhrase: "Eat shit, loser",
		},
		{
			phrase:         "You are SO cringe bro.",
			expectedToxic:  true,
			expectedPhrase: "You are SO cringe bro.",
		},
		{
			phrase:         "You are such a loser",
			expectedToxic:  true,
			expectedPhrase: "You are such a loser",
		},
		{
			phrase:         "Can we perpetuate a myth that guys with pointy ears have giant fucking dongs or some shit?",
			expectedToxic:  false,
			expectedPhrase: "",
		},
		{
			phrase:         "Fuck, dude. Oh, i fucking love moss. Look at this shit.",
			expectedToxic:  false,
			expectedPhrase: "",
		},
		{
			phrase:         "I am actually boosted. Never mind, i'm god, i'm god",
			expectedToxic:  false,
			expectedPhrase: "",
		},
		{
			phrase:         "Clobbered! Oh my fucking god, that's a quick one. Yeah, you can't do that. Yeah, I can't do that. Um... Did he just steal- Did he just pickpocket me?",
			expectedToxic:  false,
			expectedPhrase: "",
		},
		{
			phrase:         "*Evil laughter* I love this shit so much bro! Ohhhh... What do we call this? What do we call this? There has to be a name. We have to. We have to have a name for this shit.",
			expectedToxic:  false,
			expectedPhrase: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.phrase, func(t *testing.T) {
			toxicPhrase, isToxic, err := client.CheckToxicity(context.Background(), tt.phrase, true)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedToxic, isToxic)
			assert.Equal(t, tt.expectedPhrase, toxicPhrase)

			time.Sleep(3 * time.Second)
		})
	}
}
