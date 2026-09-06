Lieu : {{ .Lieu.Nom }} ({{ .Lieu.Region }})
{{ .Lieu.Resume }}

Fait historique à raconter — « {{ .Fait.Titre }} » ({{ .Fait.Epoque }})

{{ .Fait.Recit }}
{{ if .Fait.Personnages }}
Personnages historiques concernés : {{ range $i, $p := .Fait.Personnages }}{{ if $i }}, {{ end }}{{ $p }}{{ end }}.
Rappel : ils ne sont jamais locuteurs, seulement cités ou imités par le conteur.
{{ end }}{{ if .Angle }}
Angle à privilégier : {{ .Angle }}
{{ end }}
Écris l'intro puis l'histoire.
