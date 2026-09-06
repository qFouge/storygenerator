package pipeline

import (
	"strings"
	"testing"
)

var casting = Famille{Nom: "Test", Membres: []Personnage{
	{ID: "papy", Nom: "Henri", Age: 72},
	{ID: "lea", Nom: "Léa", Age: 11},
	{ID: "tom", Nom: "Tom", Age: 7},
}}

// scriptValide fabrique un script qui passe tous les contrôles, que chaque
// test dégrade ensuite sur un seul point.
func scriptValide() Script {
	return Script{
		Titre:   "Un escalier bizarre",
		Conteur: "papy",
		Intro: []Replique{
			{Personnage: "tom", Texte: "C'est quoi ce truc ?"},
			{Personnage: "lea", Texte: "Un château, évidemment."},
			{Personnage: "papy", Texte: "Figurez-vous que j'ai une histoire là-dessus."},
		},
		Histoire: []Replique{
			{Personnage: "papy", Texte: "Le roi arrive devant les marais."},
			{Personnage: "tom", Texte: "Il avait une épée ?"},
			{Personnage: "papy", Texte: "Bien sûr qu'il avait une épée."},
			{Personnage: "lea", Texte: "Et comment on sait tout ça ?"},
			{Personnage: "papy", Texte: "Justement, on ne le sait pas vraiment."},
			{Personnage: "tom", Texte: "Alors t'inventes !"},
			{Personnage: "lea", Texte: "Non, il a dit qu'on ne savait pas. C'est différent."},
		},
	}
}

func TestValiderAccepteUnScriptCorrect(t *testing.T) {
	if pbs := Valider(scriptValide(), casting); len(pbs) != 0 {
		t.Fatalf("attendu aucune anomalie, obtenu %v", pbs)
	}
}

func TestValiderDetecte(t *testing.T) {
	cas := []struct {
		nom      string
		degrader func(*Script)
		attendu  string
	}{
		{
			nom:      "locuteur hors casting",
			degrader: func(s *Script) { s.Histoire[0].Personnage = "francois-1er" },
			attendu:  "hors casting",
		},
		{
			nom:      "conteur absent de l'intro",
			degrader: func(s *Script) { s.Intro = s.Intro[:2] },
			attendu:  "ne parle pas dans l'intro",
		},
		{
			nom: "conteur effacé de l'histoire",
			degrader: func(s *Script) {
				for i := range s.Histoire {
					if s.Histoire[i].Personnage == "papy" {
						s.Histoire[i].Personnage = "lea"
					}
				}
			},
			attendu: "ne porte que",
		},
		{
			nom: "enfant quasi muet",
			degrader: func(s *Script) {
				for i := range s.Histoire {
					if s.Histoire[i].Personnage == "tom" {
						s.Histoire[i].Personnage = "papy"
					}
				}
			},
			attendu: "réplique(s) dans l'histoire",
		},
		{
			nom:      "réplique vide",
			degrader: func(s *Script) { s.Histoire[1].Texte = "   " },
			attendu:  "texte vide",
		},
		{
			nom:      "réplique trop longue",
			degrader: func(s *Script) { s.Histoire[0].Texte = strings.Repeat("a", MaxCaracteresReplique+1) },
			attendu:  "caractères, max",
		},
		{
			nom:      "conteur inconnu",
			degrader: func(s *Script) { s.Conteur = "leonard" },
			attendu:  "conteur inconnu",
		},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			s := scriptValide()
			c.degrader(&s)
			pbs := Valider(s, casting)
			for _, p := range pbs {
				if strings.Contains(p, c.attendu) {
					return
				}
			}
			t.Fatalf("attendu une anomalie contenant %q, obtenu %v", c.attendu, pbs)
		})
	}
}

// Le schéma est le seul rempart empêchant un personnage historique de
// devenir un locuteur : on vérifie que l'enum est bien bâti sur le casting.
func TestSchemaContraintLesLocuteurs(t *testing.T) {
	schema := SchemaScript(casting)
	props := schema["properties"].(map[string]any)

	conteur := props["conteur"].(map[string]any)
	if got := conteur["enum"].([]string); len(got) != 3 || got[0] != "papy" {
		t.Fatalf("enum conteur inattendu : %v", got)
	}

	intro := props["intro"].(map[string]any)
	replique := intro["items"].(map[string]any)
	perso := replique["properties"].(map[string]any)["personnage"].(map[string]any)
	if got := perso["enum"].([]string); len(got) != 3 {
		t.Fatalf("enum personnage inattendu : %v", got)
	}
	if replique["additionalProperties"] != false {
		t.Fatal("additionalProperties doit être false pour un structured output")
	}
}
