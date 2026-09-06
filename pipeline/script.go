package pipeline

// Replique est une unité de parole : un locuteur, un texte.
// C'est exactement l'unité envoyée à ElevenLabs (une requête = une réplique,
// avec la voix du personnage).
type Replique struct {
	Personnage string `json:"personnage"`
	Texte      string `json:"texte"`
}

// Script est la sortie complète d'une génération.
type Script struct {
	Titre    string     `json:"titre"`
	Conteur  string     `json:"conteur"`
	Intro    []Replique `json:"intro"`
	Histoire []Replique `json:"histoire"`
}

// Tout renvoie les répliques dans l'ordre de lecture, intro puis histoire.
func (s Script) Tout() []Replique {
	return append(append([]Replique{}, s.Intro...), s.Histoire...)
}

// SchemaScript construit le JSON Schema passé à l'API en structured output.
//
// Le point clé : `personnage` et `conteur` sont des enums bâtis sur les IDs
// de la famille. Le modèle ne peut donc pas produire un locuteur inconnu —
// ni un personnage historique, ni un membre inventé. La contrainte est
// appliquée côté serveur, pas espérée dans le prompt.
func SchemaScript(fam Famille) map[string]any {
	ids := fam.IDs()

	replique := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"personnage", "texte"},
		"properties": map[string]any{
			"personnage": map[string]any{
				"type":        "string",
				"enum":        ids,
				"description": "ID du membre de la famille qui parle. Jamais un personnage historique.",
			},
			"texte": map[string]any{
				"type":        "string",
				"description": "Texte prononcé d'une traite, 300 caractères maximum.",
			},
		},
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"titre", "conteur", "intro", "histoire"},
		"properties": map[string]any{
			"titre": map[string]any{
				"type":        "string",
				"description": "Titre court de l'épisode.",
			},
			"conteur": map[string]any{
				"type":        "string",
				"enum":        ids,
				"description": "Le membre qui raconte. Le même dans l'intro et dans l'histoire.",
			},
			"intro":    map[string]any{"type": "array", "minItems": 8, "items": replique},
			"histoire": map[string]any{"type": "array", "minItems": 20, "items": replique},
		},
	}
}
