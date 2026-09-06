> **Superseded (September 2026).** This documents the road-trip / Loire-châteaux
> direction, generated with the Anthropic API in French. The project was
> re-briefed around generic multi-character audio stories, Gemini for script
> generation and ElevenLabs Text-to-Dialogue for audio. Kept as a record of a
> direction that was abandoned, and of the reasoning behind it. Current brief:
> [brief-dialogue.md](brief-dialogue.md). Current decisions:
> [text-to-dialogue.md](text-to-dialogue.md).

# Brief projet — archive

Brief fourni à l'initialisation du repo, conservé tel quel comme référence
historique. Les décisions prises en réponse sont dans
[coherence-multi-personnages.md](coherence-multi-personnages.md) ; l'état
courant du projet est dans le [README](../README.md).

> Note : le point 5 du brief (« stratégie de température/randomisation ») repose
> sur une prémisse devenue fausse — `temperature` n'existe plus sur les modèles
> Claude actuels. Voir la note de conception.

---

## Générateur d'histoires audio pour road trips familiaux

### Contexte général

Application qui génère des histoires audio captivantes basées sur des lieux
historiques (châteaux de la Loire pour commencer), destinées à être écoutées en
voiture par des familles. Chaque histoire met en scène une famille fictive
(parents, enfants, grands-parents) dans une intro, où un personnage devient
« conteur » et raconte un fait historique réel de manière dramatisée. Le texte
final est converti en audio multi-voix (une voix différente par personnage) via
ElevenLabs.

### Objectif de cette première session

Initialiser le repo et poser les bases techniques du pipeline de génération de
texte, en se concentrant d'abord sur la cohérence narrative multi-personnages
avant l'intégration audio complète.

### Structure du pipeline validée

1. Input manuel : lieu + données historiques vérifiées (JSON/YAML)
2. Génération de l'intro (famille en voiture découvre le lieu) — format `[PERSONNAGE] "dialogue"`
3. Génération de l'histoire principale (extension du fait historique, avec suspense et dialogues)
4. Vérification de sécurité (contenu adapté aux enfants)
5. Découpage du texte par personnage → appel ElevenLabs par segment avec voix assignée
6. Assemblage audio final (pas de streaming temps réel — génération complète puis écoute)

### À initialiser dans le repo

- Structure de dossiers : `/prompts`, `/data/lieux` (un fichier par lieu, ex. `chambord.yaml`), `/pipeline`, `/audio`
- Le prompt générique et les données Chambord comme premiers fichiers de config
- Fichier `.env.example` pour les clés API (Anthropic + ElevenLabs), sans jamais committer de vraie clé
- README décrivant l'architecture du pipeline

### Point à creuser en priorité : cohérence de la génération multi-personnages

- Comment garantir que le LLM garde des voix/personnalités cohérentes entre
  l'intro et l'histoire (ex. le grand-père reste le conteur, les enfants posent
  des questions cohérentes avec leur âge) ?
- Faut-il générer l'intro et l'histoire en un seul appel ou en deux appels
  séparés (avec le risque de rupture de ton entre les deux) ?
- Comment gérer le nommage des personnages historiques réels (François Ier,
  Molière) sans les confondre avec les personnages de la famille dans le
  parsing par `[PERSONNAGE]` ?
- Faut-il un schéma de validation (ex. regex ou parsing JSON structuré plutôt
  que du texte brut avec des tags) pour fiabiliser l'extraction avant l'envoi à
  ElevenLabs ?
- Stratégie de température/randomisation : comment varier les personnages qui
  interviennent sans casser la cohérence factuelle ?

### Ce qui n'est pas encore à traiter

- Base de données de lieux (viendra plus tard)
- Interface utilisateur
- Génération audio en elle-même (à faire une fois le texte fiabilisé)

---

## Écarts entre le brief et ce qui a été implémenté

| Brief | Implémenté | Pourquoi |
|---|---|---|
| Format `[PERSONNAGE] "dialogue"` | JSON contraint par schéma | Le parsing par tags casse sur le discours rapporté et les personnages historiques |
| Étapes 2 et 3 séparées | Un seul appel produisant les deux | Supprime la rupture de ton, coût identique |
| Langage non spécifié | Go | Choix de session : backend seul, et volonté d'apprendre Go |
| Prompt et données Chambord « fournis » | Rédigés de zéro | Les documents n'ont pas été transmis ; les faits Chambord sont marqués `verifie: false` et bloquent la génération tant qu'ils ne sont pas recoupés |
