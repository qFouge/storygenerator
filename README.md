# storygenerator

Générateur d'histoires audio pour road trips familiaux. Chaque épisode met en
scène une famille fictive en voiture qui découvre un lieu historique ; l'un des
personnages devient conteur et raconte un fait réel, dramatisé. Le script est
ensuite découpé par personnage et lu par une voix distincte via ElevenLabs.

**État actuel : pipeline texte uniquement.** L'étape audio n'est pas encore
implémentée (voir « Ce qui n'existe pas encore »).

## Démarrage

```bash
cp .env.example .env        # renseigner ANTHROPIC_API_KEY
go test ./...
go run ./cmd/generate -lieu chambord -dry-run   # résout les entrées, n'appelle pas l'API
go run ./cmd/generate -lieu chambord            # génère out/chambord-<fait>.json
```

`-fait <id>` force un fait précis, `-graine <n>` rend le tirage reproductible.

Un lieu ne produit rien tant qu'aucun de ses faits n'est marqué
`verifie: true` : les données de départ de `data/lieux/chambord.yaml` sont
rédigées de mémoire et doivent être recoupées avant usage.

## Architecture du pipeline

```
data/famille.yaml ─┐
data/lieux/*.yaml ─┴─► [1] chargement + sélection du fait   pipeline/config.go, cmd/generate
                              │
prompts/*.md ────────► [2] rendu des prompts (text/template) pipeline/generer.go
                              │
                       [3] UN appel Claude, sortie JSON      pipeline/generer.go
                           contrainte par schéma             pipeline/script.go
                              │  { titre, conteur, intro[], histoire[] }
                              │
                       [4] validation de cohérence           pipeline/valider.go
                              │
                       [5] relecture sécurité enfants        pipeline/securite.go
                              │  (second appel, modèle distinct)
                              ▼
                       out/<lieu>-<fait>.json
                              │
                       [6] TTS par réplique  ── pas encore implémenté
                       [7] assemblage audio  ── pas encore implémenté
```

### Les trois décisions structurantes

**Un seul appel pour l'intro et l'histoire.** Le modèle voit tout l'arc d'un
coup : pas de rupture de ton entre les deux parties, et le conteur est désigné
une fois pour toutes.

**Sortie JSON contrainte par schéma, pas des tags `[PERSONNAGE]` en texte
brut.** Le champ `personnage` est un `enum` construit à l'exécution sur les IDs
de `data/famille.yaml`. Un personnage historique ne *peut pas* devenir
locuteur — la contrainte est appliquée par l'API, pas espérée dans le prompt.
François Ier parle uniquement en discours rapporté, à l'intérieur d'une
réplique du conteur.

**Le casting est une donnée, pas de la prose.** Le même fichier alimente le
prompt système *et* l'enum du schéma *et* la validation *et*, plus tard, la
table des voix ElevenLabs. Une seule source de vérité pour la cohérence.

Le raisonnement complet, y compris ce qui a été écarté, est dans
[docs/coherence-multi-personnages.md](docs/coherence-multi-personnages.md).

## Organisation des fichiers

| Chemin | Rôle |
|---|---|
| `data/famille.yaml` | Le casting récurrent : caractère, façon de parler, voix ElevenLabs |
| `data/lieux/<id>.yaml` | Un lieu et ses faits historiques sourcés |
| `prompts/systeme.md` | Casting + règles de cohérence. Stable ⇒ mis en cache par l'API |
| `prompts/utilisateur.md` | Le lieu et le fait du jour. C'est ce qui varie |
| `pipeline/` | Chargement, schéma, génération, validation, sécurité |
| `cmd/generate/` | CLI |
| `out/` | Scripts générés (ignoré par git) |
| `audio/` | Sortie audio à venir (ignoré par git) |
| `docs/` | Décisions de conception et archives |

## Ce qui n'existe pas encore

- **Étapes 6 et 7** : appel ElevenLabs par réplique et assemblage. Le format de
  sortie est déjà l'unité d'envoi — une réplique = une requête TTS = une voix.
- Base de données de lieux : un fichier YAML par lieu suffit pour l'instant.
- Interface : aucune. Si une interface arrive un jour, elle sera mobile-first.
- Boucle de régénération automatique quand `Valider` remonte des anomalies :
  aujourd'hui elles sont affichées sur stderr, le script est écrit quand même.

## Sécurité

`.env` est ignoré par git ; `.env.example` ne contient que des placeholders.
Aucune clé ne doit apparaître dans un fichier suivi, un prompt ou un YAML.
