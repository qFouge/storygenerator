// Commande generate : produit le script texte d'une histoire et l'écrit en JSON.
//
//	go run ./cmd/generate -lieu chambord
//	go run ./cmd/generate -lieu chambord -fait escalier-double-revolution
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/qdefou/storygenerator/pipeline"
)

func main() {
	var (
		lieuID  = flag.String("lieu", "", "identifiant du lieu (nom du fichier dans data/lieux)")
		faitID  = flag.String("fait", "", "identifiant du fait ; vide = tirage parmi les faits vérifiés")
		graine  = flag.Int64("graine", 0, "graine du tirage ; 0 = horloge")
		dossier = flag.String("data", "data", "dossier des données")
		sortie  = flag.String("out", "out", "dossier de sortie")
		sansIA  = flag.Bool("dry-run", false, "afficher les entrées résolues sans appeler l'API")
	)
	flag.Parse()

	if *lieuID == "" {
		log.Fatal("-lieu est requis")
	}

	famille, err := pipeline.ChargerYAML[pipeline.Famille](filepath.Join(*dossier, "famille.yaml"))
	if err != nil {
		log.Fatal(err)
	}
	lieu, err := pipeline.ChargerYAML[pipeline.Lieu](filepath.Join(*dossier, "lieux", *lieuID+".yaml"))
	if err != nil {
		log.Fatal(err)
	}

	fait, angle, err := choisir(lieu, *faitID, *graine)
	if err != nil {
		log.Fatal(err)
	}

	demande := pipeline.Demande{Lieu: lieu, Fait: fait, Famille: famille, Angle: angle}

	if *sansIA {
		fmt.Printf("lieu    : %s\nfait    : %s (%s)\nangle   : %s\ncasting : %v\n",
			lieu.Nom, fait.Titre, fait.ID, angle, famille.IDs())
		return
	}

	ctx := context.Background()
	client := anthropic.NewClient() // lit ANTHROPIC_API_KEY

	script, err := pipeline.Generer(ctx, client, demande)
	if err != nil {
		log.Fatal(err)
	}

	if pbs := pipeline.Valider(script, famille); len(pbs) > 0 {
		fmt.Fprintln(os.Stderr, "cohérence — anomalies :")
		for _, p := range pbs {
			fmt.Fprintln(os.Stderr, "  -", p)
		}
	}

	verdict, err := pipeline.VerifierSecurite(ctx, client, script)
	if err != nil {
		log.Fatal(err)
	}
	if !verdict.Adapte {
		fmt.Fprintln(os.Stderr, "sécurité — script refusé :")
		for i, m := range verdict.Motifs {
			fmt.Fprintf(os.Stderr, "  - %s\n", m)
			if i < len(verdict.Extraits) {
				fmt.Fprintf(os.Stderr, "    %q\n", verdict.Extraits[i])
			}
		}
	}

	if err := os.MkdirAll(*sortie, 0o755); err != nil {
		log.Fatal(err)
	}
	chemin := filepath.Join(*sortie, fmt.Sprintf("%s-%s.json", lieu.ID, fait.ID))
	brut, err := json.MarshalIndent(script, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(chemin, brut, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s — conteur %s, %d répliques → %s\n",
		script.Titre, script.Conteur, len(script.Tout()), chemin)
}

// choisir résout le fait et l'angle. Sans -fait, il tire parmi les faits
// vérifiés : c'est le levier de variété entre deux exécutions, la cohérence
// factuelle restant garantie par les données d'entrée.
func choisir(l pipeline.Lieu, faitID string, graine int64) (pipeline.Fait, string, error) {
	if graine == 0 {
		graine = time.Now().UnixNano()
	}
	tirage := rand.New(rand.NewSource(graine))

	if faitID != "" {
		f, ok := l.Fait(faitID)
		if !ok {
			return f, "", fmt.Errorf("fait %q introuvable dans %s", faitID, l.ID)
		}
		if !f.Verifie {
			return f, "", fmt.Errorf("fait %q non vérifié : compléter `source` et passer `verifie: true`", faitID)
		}
		return f, angleAleatoire(f, tirage), nil
	}

	verifies := l.FaitsVerifies()
	if len(verifies) == 0 {
		return pipeline.Fait{}, "", fmt.Errorf("aucun fait vérifié dans %s : vérifier les sources puis passer `verifie: true`", l.ID)
	}
	f := verifies[tirage.Intn(len(verifies))]
	return f, angleAleatoire(f, tirage), nil
}

func angleAleatoire(f pipeline.Fait, r *rand.Rand) string {
	if len(f.Angles) == 0 {
		return ""
	}
	return f.Angles[r.Intn(len(f.Angles))]
}
