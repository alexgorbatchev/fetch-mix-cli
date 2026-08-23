package parser

import (
	"reflect"
	"testing"

	"github.com/alexgorbatchev/fetch-mix-cli/internal/types"
)

func TestParseTracklist(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		want     []types.Track
	}{
		{
			name: "MixesDB markdown with varied formatting",
			markdown: `
# 2014-09-27 - Bicep - Essential Mix

File details
------------
This show or mix is ~2 hours long.

Tracklist
---------

1.  \[000\] Intro
2.  \[00?\] Hammer & Bicep - I Believe \[Feel My Bicep\]
3.  \[008\] Daniel Jacques - End Of My World {Mistress\]
4.  \[012\] Ray Mang & Foolish Felix - Who Said Beats? \[Eskimo\]
5.  \[0??\] Times Are Ruff - Cut2 \[Times Are Ruff\]
6.  \[020\] Herbert - People That Make The Music
7.  \[0??\] _Hammer & Bicep - Untitled \[Feel My Bicep\]_
8.  \[108\] _Groove Chronicles - Stone Cold (Bicep Edit)_?
9.  \[0??\] ID - ID
10. \[112\] EOD - Phontron (030303 Mix) \[030303 - 030EP 002\]

Related mixes
-------------
* [2012-12-24 - Bicep - Podcast 194](https://www.mixesdb.com/)
`,
			want: []types.Track{
				{Artist: "Hammer & Bicep", Title: "I Believe", RawString: "Hammer & Bicep - I Believe"},
				{Artist: "Daniel Jacques", Title: "End Of My World", RawString: "Daniel Jacques - End Of My World"},
				{Artist: "Ray Mang & Foolish Felix", Title: "Who Said Beats", RawString: "Ray Mang & Foolish Felix - Who Said Beats"},
				{Artist: "Times Are Ruff", Title: "Cut2", RawString: "Times Are Ruff - Cut2"},
				{Artist: "Herbert", Title: "People That Make The Music", RawString: "Herbert - People That Make The Music"},
				{Artist: "Hammer & Bicep", Title: "Untitled", RawString: "Hammer & Bicep - Untitled"},
				{Artist: "Groove Chronicles", Title: "Stone Cold (Bicep Edit)", RawString: "Groove Chronicles - Stone Cold (Bicep Edit)"},
				{Artist: "EOD", Title: "Phontron (030303 Mix)", RawString: "EOD - Phontron (030303 Mix)"},
			},
		},
		{
			name:     "No tracklist section",
			markdown: "No tracks here, just some info.",
			want:     nil,
		},
		{
			name: "Brizm style tracklists with em-dashes and spaced lines",
			markdown: `
Polo & Pan live for Cercle Tracklist
-----------------------------------

Tracklist
---------

Export ▾

1

Vladimir Cosma — Sirba (Polo & Pan Edit)

98 BPM A · 11B

2

Barbatuques — Baião Destemperado (Polo & Pan Edit) ▶ 2:00

100 BPM C#m · 12A

3

Polo & Pan — Zoom Zoom ▶ 3:00

Similar Sets
------------
`,
			want: []types.Track{
				{Artist: "Vladimir Cosma", Title: "Sirba (Polo & Pan Edit)", RawString: "Vladimir Cosma - Sirba (Polo & Pan Edit)"},
				{Artist: "Barbatuques", Title: "Baião Destemperado (Polo & Pan Edit)", RawString: "Barbatuques - Baião Destemperado (Polo & Pan Edit)"},
				{Artist: "Polo & Pan", Title: "Zoom Zoom", RawString: "Polo & Pan - Zoom Zoom"},
			},
		},
		{
			name: "OpeningTrack style tracklists with en-dashes and inline links",
			markdown: `
Polo & Pan @ Serre Monumentale for Cercle 09.05.2018
----------------------------------------------------

Tracklist  
  
Intro : Vladimir Cosma – Sirba (Le Grand Blond avec une Chaussure Noire)  
[02:00](https://www.youtube.com/watch?v=CsGauHXioos&t=120s)
 Barbatuques – Baião Destemperado  
[03:00](https://www.youtube.com/watch?v=CsGauHXioos&t=180s)
 Polo & Pan – Zoom Zoom  
[07:00](https://www.youtube.com/watch?v=CsGauHXioos&t=420s)
 Banda Ionica – Lorenzo in Sicilia  

Download: zippyshare
`,
			want: []types.Track{
				{Artist: "Vladimir Cosma", Title: "Sirba (Le Grand Blond avec une Chaussure Noire)", RawString: "Vladimir Cosma - Sirba (Le Grand Blond avec une Chaussure Noire)"},
				{Artist: "Barbatuques", Title: "Baião Destemperado", RawString: "Barbatuques - Baião Destemperado"},
				{Artist: "Polo & Pan", Title: "Zoom Zoom", RawString: "Polo & Pan - Zoom Zoom"},
				{Artist: "Banda Ionica", Title: "Lorenzo in Sicilia", RawString: "Banda Ionica - Lorenzo in Sicilia"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := ParseTracklist(tt.markdown)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseTracklist() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
