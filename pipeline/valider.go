package pipeline

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxCaracteresReplique plafonne la longueur d'une réplique.
// ponytail: plafond conservateur choisi pour le confort d'écoute, pas calé
// sur la limite réelle d'ElevenLabs — à ajuster quand l'audio sera branché.
const MaxCaracteresReplique = 400

// Valider vérifie ce que le schéma JSON ne peut pas garantir : la cohérence
// narrative. Le schéma garantit la *forme* (locuteurs connus, champs présents),
// cette fonction garantit le *fond* (le conteur raconte vraiment, les enfants
// existent, rien n'est vide).
//
// Elle renvoie toutes les anomalies trouvées plutôt que la première, pour
// qu'une régénération puisse corriger l'ensemble d'un coup.
func Valider(s Script, fam Famille) []string {
	var pbs []string

	if _, ok := fam.Membre(s.Conteur); !ok {
		pbs = append(pbs, fmt.Sprintf("conteur inconnu : %q", s.Conteur))
	}
	if len(s.Intro) == 0 || len(s.Histoire) == 0 {
		pbs = append(pbs, "intro ou histoire vide")
		return pbs
	}

	// Ceinture et bretelles : l'enum du schéma devrait déjà l'interdire, mais
	// une sortie de modèle est une frontière de confiance — on revérifie avant
	// de router quoi que ce soit vers une voix.
	compte := map[string]int{}
	for i, r := range s.Tout() {
		if _, ok := fam.Membre(r.Personnage); !ok {
			pbs = append(pbs, fmt.Sprintf("réplique %d : locuteur hors casting %q", i, r.Personnage))
		}
		if strings.TrimSpace(r.Texte) == "" {
			pbs = append(pbs, fmt.Sprintf("réplique %d : texte vide", i))
		}
		if n := utf8.RuneCountInString(r.Texte); n > MaxCaracteresReplique {
			pbs = append(pbs, fmt.Sprintf("réplique %d (%s) : %d caractères, max %d",
				i, r.Personnage, n, MaxCaracteresReplique))
		}
		compte[r.Personnage]++
	}

	// Le conteur doit exister des deux côtés : c'est le fil qui empêche
	// l'intro et l'histoire de se comporter comme deux textes différents.
	if !parle(s.Intro, s.Conteur) {
		pbs = append(pbs, fmt.Sprintf("le conteur %q ne parle pas dans l'intro", s.Conteur))
	}
	if part := partDuConteur(s.Histoire, s.Conteur); part < 0.4 {
		pbs = append(pbs, fmt.Sprintf("le conteur %q ne porte que %.0f%% de l'histoire (minimum 40%%)",
			s.Conteur, part*100))
	}

	// Chaque enfant doit exister dans les deux parties, sinon on obtient un
	// monologue de grand-père avec deux enfants décoratifs.
	for _, m := range fam.Membres {
		if m.Age >= 15 {
			continue
		}
		if !parle(s.Intro, m.ID) {
			pbs = append(pbs, fmt.Sprintf("%s (%d ans) ne parle pas dans l'intro", m.Nom, m.Age))
		}
		if n := compteDans(s.Histoire, m.ID); n < 2 {
			pbs = append(pbs, fmt.Sprintf("%s (%d ans) n'a que %d réplique(s) dans l'histoire (minimum 2)",
				m.Nom, m.Age, n))
		}
	}

	return pbs
}

func parle(rs []Replique, id string) bool { return compteDans(rs, id) > 0 }

func compteDans(rs []Replique, id string) int {
	n := 0
	for _, r := range rs {
		if r.Personnage == id {
			n++
		}
	}
	return n
}

func partDuConteur(rs []Replique, id string) float64 {
	if len(rs) == 0 {
		return 0
	}
	return float64(compteDans(rs, id)) / float64(len(rs))
}
