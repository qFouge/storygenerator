package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/anthropics/anthropic-sdk-go"
)

// ModeleRedaction est le modèle qui écrit l'histoire.
const ModeleRedaction = "claude-opus-5"

// Demande rassemble tout ce dont une génération a besoin.
type Demande struct {
	Lieu    Lieu
	Fait    Fait
	Famille Famille
	// Angle est l'un des angles du fait. Vide = laisser le modèle choisir.
	Angle string
	// Variation est une consigne de mise en scène ajoutée au prompt système.
	// C'est le levier de diversité entre deux histoires du même fait
	// (cf. docs/coherence-multi-personnages.md).
	Variation string
	// DossierPrompts contient systeme.md et utilisateur.md. Vide = "prompts".
	DossierPrompts string
}

// Generer produit intro et histoire en un seul appel.
//
// Un seul appel, volontairement : le modèle voit tout l'arc narratif d'un
// coup, ce qui supprime la rupture de ton entre les deux parties et lui
// permet de désigner le conteur une fois pour toutes.
func Generer(ctx context.Context, client anthropic.Client, d Demande) (Script, error) {
	var s Script

	dossier := d.DossierPrompts
	if dossier == "" {
		dossier = "prompts"
	}
	systeme, err := rendre(filepath.Join(dossier, "systeme.md"), d)
	if err != nil {
		return s, err
	}
	utilisateur, err := rendre(filepath.Join(dossier, "utilisateur.md"), d)
	if err != nil {
		return s, err
	}

	adaptatif := anthropic.ThinkingConfigAdaptiveParam{}
	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     ModeleRedaction,
		MaxTokens: 16000,
		Thinking:  anthropic.ThinkingConfigParamUnion{OfAdaptive: &adaptatif},
		// Le prompt système (casting + règles) ne change pas d'une histoire à
		// l'autre : on le met en cache. Ce qui varie (lieu, fait) est dans le
		// message utilisateur, donc après le point de césure du cache.
		System: []anthropic.TextBlockParam{{
			Text:         systeme,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(utilisateur)),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: SchemaScript(d.Famille)},
		},
	})
	if err != nil {
		return s, fmt.Errorf("appel modèle : %w", err)
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return s, fmt.Errorf("refus du modèle (%s) : %s", resp.StopDetails.Category, resp.StopDetails.Explanation)
	}

	brut, err := texteDe(resp)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal([]byte(brut), &s); err != nil {
		return s, fmt.Errorf("décodage du script : %w", err)
	}
	return s, nil
}

// texteDe extrait le bloc texte de la réponse. Les blocs de raisonnement
// précèdent le texte, d'où le parcours plutôt qu'un accès à Content[0].
func texteDe(resp *anthropic.Message) (string, error) {
	for _, bloc := range resp.Content {
		if t, ok := bloc.AsAny().(anthropic.TextBlock); ok {
			return t.Text, nil
		}
	}
	return "", fmt.Errorf("aucun bloc texte dans la réponse (stop_reason=%s)", resp.StopReason)
}

func rendre(chemin string, d Demande) (string, error) {
	brut, err := os.ReadFile(chemin)
	if err != nil {
		return "", fmt.Errorf("lecture prompt : %w", err)
	}
	tpl, err := template.New(filepath.Base(chemin)).Parse(string(brut))
	if err != nil {
		return "", fmt.Errorf("parsing prompt %s : %w", chemin, err)
	}
	var sb strings.Builder
	if err := tpl.Execute(&sb, d); err != nil {
		return "", fmt.Errorf("rendu prompt %s : %w", chemin, err)
	}
	return sb.String(), nil
}
