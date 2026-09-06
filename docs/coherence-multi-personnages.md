> **Superseded (September 2026).** This documents the road-trip / Loire-châteaux
> direction, generated with the Anthropic API in French. The project was
> re-briefed around generic multi-character audio stories, Gemini for script
> generation and ElevenLabs Text-to-Dialogue for audio. Kept as a record of a
> direction that was abandoned, and of the reasoning behind it. Current brief:
> [brief-dialogue.md](brief-dialogue.md). Current decisions:
> [text-to-dialogue.md](text-to-dialogue.md).

# Cohérence de la génération multi-personnages

Note de conception, session d'initialisation du repo. Répond aux cinq questions
ouvertes du brief initial ([docs/brief-initial.md](brief-initial.md)).

---

## 1. Garder des voix et des personnalités stables entre l'intro et l'histoire

**Décision : le casting est une donnée structurée, pas une description dans le
prompt.**

`data/famille.yaml` décrit chaque membre par un `id`, un âge, un caractère et
une façon de parler. Ce fichier unique alimente quatre choses :

1. le prompt système, où chaque personnage est présenté ;
2. l'`enum` du schéma JSON de sortie, bâti sur les `id` ;
3. la validation post-génération ;
4. plus tard, la table des voix ElevenLabs (`voix_id`).

Une seule source de vérité, donc pas de dérive possible entre ce que le modèle
lit, ce qu'il a le droit de produire, et ce qu'on vérifie ensuite.

**Le conteur est un champ obligatoire du schéma**, contraint au même `enum`. Le
modèle doit s'engager explicitement sur un conteur avant d'écrire, plutôt que
de le laisser émerger — et `pipeline/valider.go` vérifie ensuite que ce conteur
parle bien dans l'intro et porte au moins 40 % des répliques de l'histoire.
C'est ce qui empêche « le grand-père commence, quelqu'un d'autre finit ».

La cohérence par âge est traitée deux fois, par le prompt (« Tom, sept ans,
demande des choses concrètes ; Léa, onze ans, demande comment on le sait ») et
par la validation (chaque enfant a au moins une réplique dans l'intro et deux
dans l'histoire). Le prompt oriente, la validation constate.

## 2. Un appel ou deux ?

**Décision : un seul appel produisant intro et histoire ensemble.**

Deux appels créent exactement le problème redouté : le second ne partage ni le
raisonnement ni le rythme du premier, et il faut réinjecter l'intro en contexte
en espérant que le ton suive. Un seul appel donne au modèle l'arc complet, ce
qui lui permet de placer les questions de l'intro en préparation de l'histoire.

Le coût est quasiment identique : ce sont les mêmes tokens de sortie, et le
prompt système n'est facturé qu'une fois au lieu de deux.

Le prix à payer : on ne peut pas régénérer l'intro seule. Ce n'est pas gênant
aujourd'hui — une génération complète coûte un appel. Si le besoin apparaît, la
réponse ne sera pas de découper en deux appels mais de rappeler le modèle avec
le script existant et une consigne de réécriture ciblée : le contexte reste
complet, c'est ce qui compte.

## 3. Personnages historiques contre personnages de la famille

**Décision : un personnage historique n'est jamais un locuteur.**

Le champ `personnage` d'une réplique est un `enum` limité aux `id` de la
famille. L'API rejette toute autre valeur : ce n'est pas une consigne que le
modèle peut oublier, c'est une contrainte de décodage.

François Ier, Molière, Léonard de Vinci apparaissent donc uniquement **en
discours rapporté à l'intérieur d'une réplique du conteur** — le conteur les
imite, comme un grand-père le ferait réellement en voiture :

```json
{ "personnage": "papy",
  "texte": "Et là le roi se retourne et lui dit : « Ce sera ici, et nulle part ailleurs. »" }
```

Ce choix rend le découpage pour ElevenLabs trivial : une réplique = une requête
= une voix, sans ambiguïté possible.

*Écarté pour l'instant :* un champ optionnel `incarne` qui donnerait une voix
distincte aux figures historiques. C'est faisable — ajouter le champ au schéma
et une seconde table de voix — mais ça double la complexité du mapping audio
pour un gain qui n'a pas encore été entendu. À rouvrir une fois que les
premières histoires auront été écoutées en vrai.

## 4. Schéma de validation

**Décision : structured outputs (`output_config.format`) plutôt qu'un parsing
de texte balisé.**

Une regex sur `[PERSONNAGE]` casse sur les cas qui vont forcément arriver :
crochets à l'intérieur d'une réplique, dialogue rapporté, nom composé, ligne
sans balise. Le schéma JSON supprime la classe entière de problèmes.

Le schéma garantit la **forme**. `pipeline/valider.go` vérifie le **fond**, ce
qu'un schéma ne sait pas exprimer :

- le conteur parle dans l'intro et porte ≥ 40 % de l'histoire ;
- chaque enfant a ≥ 1 réplique dans l'intro et ≥ 2 dans l'histoire ;
- aucune réplique vide, aucune au-delà de 400 caractères ;
- re-contrôle des locuteurs — redondant avec l'`enum`, gardé quand même : une
  sortie de modèle est une frontière de confiance, et ce qu'on route ensuite
  vers une synthèse vocale mérite une seconde barrière.

La validation renvoie *toutes* les anomalies, pas la première, pour qu'une
future boucle de régénération puisse tout corriger en un tour.

## 5. Variation sans perte de cohérence factuelle

**Le paramètre `temperature` n'existe plus.** Il est supprimé sur les modèles
actuels (Opus 5, Sonnet 5, Fable 5) et l'envoyer retourne une erreur 400. La
stratégie de randomisation ne peut donc pas passer par là — c'est une prémisse
du brief initial qu'il faut abandonner.

**Décision : la variation vient des entrées, pas du décodage.**

Deux leviers, tous deux dans les données :

- **le fait** : sans `-fait`, le CLI tire au sort parmi les faits *vérifiés* du
  lieu. Chaque lieu porte plusieurs faits, donc plusieurs histoires possibles ;
- **l'angle** : chaque fait porte une liste d'`angles`, dont un est tiré et
  injecté dans le prompt utilisateur. Même fait, cadrage différent.

Un champ `Variation` sur `pipeline.Demande` permet en plus d'ajouter une
consigne de mise en scène libre au prompt système (« l'histoire commence par la
fin », « c'est Léa qui raconte cette fois »).

L'avantage de ce placement : ce qui varie est *choisi par nous et tracé*, tandis
que les faits restent épinglés dans le YAML. La diversité augmente sans que la
fidélité historique bouge. `-graine` rend n'importe quel tirage reproductible,
ce qui permet de rejouer exactement une génération qui a mal tourné.

---

## Ce qui reste ouvert

- **Boucle de régénération.** Aujourd'hui les anomalies de `Valider` sont
  affichées et le script écrit quand même. Le prochain pas naturel est de
  renvoyer la liste d'anomalies au modèle avec le script pour correction ciblée.
- **Le seuil de 40 %** pour la part du conteur est un réglage à l'aveugle. Il
  faudra l'ajuster après avoir lu une dizaine de scripts réels.
- **Vérifier si le modèle sur-utilise le même conteur.** Si les cinq membres
  peuvent conter mais que le grand-père sort systématiquement, il faudra
  imposer le conteur en entrée plutôt que le laisser au modèle.
