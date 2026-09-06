package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

// ModeleSecurite relit le script. Un modèle moins cher suffit : la tâche est
// une classification, pas de l'écriture.
const ModeleSecurite = "claude-sonnet-5"

// Verdict est le résultat du contrôle enfants.
type Verdict struct {
	Adapte   bool     `json:"adapte"`
	Motifs   []string `json:"motifs"`
	Extraits []string `json:"extraits"`
}

const consigneSecurite = `Tu relis un script d'histoire audio destinée à des enfants de six ans et plus, écoutée en famille en voiture.

Signale uniquement ce qui est réellement problématique pour ce public :
violence décrite crûment, cruauté complaisante, contenu sexuel, images de
cauchemar, incitation à un comportement dangereux, propos discriminatoires.

L'évocation sobre d'une mort, d'une guerre ou d'une injustice historique est
acceptable et attendue : ne la signale pas.

Réponds "adapte": false uniquement si un passage doit être réécrit.
Dans "extraits", cite le texte exact fautif.`

// VerifierSecurite fait relire le script par un second appel.
// Séparer la relecture de l'écriture évite qu'un modèle valide sa propre copie.
func VerifierSecurite(ctx context.Context, client anthropic.Client, s Script) (Verdict, error) {
	var v Verdict

	var sb strings.Builder
	for _, r := range s.Tout() {
		fmt.Fprintf(&sb, "[%s] %s\n", r.Personnage, r.Texte)
	}

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"adapte", "motifs", "extraits"},
		"properties": map[string]any{
			"adapte":   map[string]any{"type": "boolean"},
			"motifs":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"extraits": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
	}

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     ModeleSecurite,
		MaxTokens: 2000,
		System:    []anthropic.TextBlockParam{{Text: consigneSecurite}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(sb.String())),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: schema},
		},
	})
	if err != nil {
		return v, fmt.Errorf("appel relecture : %w", err)
	}
	brut, err := texteDe(resp)
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal([]byte(brut), &v); err != nil {
		return v, fmt.Errorf("décodage verdict : %w", err)
	}
	return v, nil
}
