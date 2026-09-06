// Package pipeline contient la chaîne de génération : chargement des données
// d'entrée, appel au modèle, validation, contrôle de sécurité.
package pipeline

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

// Personnage est un membre de la famille fictive. Son ID est la clé de tout :
// il sert d'enum dans le schéma de sortie et de clé de voix côté ElevenLabs.
type Personnage struct {
	ID     string `yaml:"id"`
	Nom    string `yaml:"nom"`
	Role   string `yaml:"role"`
	Age    int    `yaml:"age"`
	Traits string `yaml:"traits"`
	Parler string `yaml:"parler"`
	VoixID string `yaml:"voix_id"`
}

// Famille est le casting récurrent, identique d'une histoire à l'autre.
type Famille struct {
	Nom     string       `yaml:"nom"`
	Membres []Personnage `yaml:"membres"`
}

// IDs renvoie les identifiants des membres, dans l'ordre du fichier.
func (f Famille) IDs() []string {
	ids := make([]string, len(f.Membres))
	for i, m := range f.Membres {
		ids[i] = m.ID
	}
	return ids
}

// Membre cherche un membre par son ID. Le second retour vaut false si absent
// (idiome Go du "comma ok", comme pour l'accès à une map).
func (f Famille) Membre(id string) (Personnage, bool) {
	for _, m := range f.Membres {
		if m.ID == id {
			return m, true
		}
	}
	return Personnage{}, false
}

// Fait est un épisode historique vérifié, unité de base d'une histoire.
type Fait struct {
	ID          string   `yaml:"id"`
	Titre       string   `yaml:"titre"`
	Recit       string   `yaml:"recit"`
	Personnages []string `yaml:"personnages"`
	Epoque      string   `yaml:"epoque"`
	Source      string   `yaml:"source"`
	Verifie     bool     `yaml:"verifie"`
	Angles      []string `yaml:"angles"`
}

// Lieu regroupe les faits rattachés à un même site.
type Lieu struct {
	ID     string `yaml:"id"`
	Nom    string `yaml:"nom"`
	Region string `yaml:"region"`
	Resume string `yaml:"resume"`
	Faits  []Fait `yaml:"faits"`
}

// Fait cherche un fait par son ID.
func (l Lieu) Fait(id string) (Fait, bool) {
	for _, f := range l.Faits {
		if f.ID == id {
			return f, true
		}
	}
	return Fait{}, false
}

// FaitsVerifies ne retient que les faits dont la source a été contrôlée.
// Générer une histoire "historique" à partir d'un fait non vérifié serait
// produire de la désinformation avec une voix rassurante — on refuse.
func (l Lieu) FaitsVerifies() []Fait {
	var out []Fait
	for _, f := range l.Faits {
		if f.Verifie {
			out = append(out, f)
		}
	}
	return out
}

// ChargerYAML lit et décode un fichier YAML dans T.
// Le paramètre de type évite d'écrire deux fois la même fonction.
func ChargerYAML[T any](chemin string) (T, error) {
	var v T
	brut, err := os.ReadFile(chemin)
	if err != nil {
		return v, fmt.Errorf("lecture %s : %w", chemin, err)
	}
	if err := yaml.Unmarshal(brut, &v); err != nil {
		return v, fmt.Errorf("décodage %s : %w", chemin, err)
	}
	return v, nil
}
