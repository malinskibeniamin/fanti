# Third-party data and licences

Fanti's original source code is licensed under the MIT License. The datasets and third-party
materials below are separate works and remain under their respective terms.

Fanti embeds, downloads, or redistributes the following datasets. Licence texts supplied
with vendored inputs remain in their archives or live beside the data under `backend/data/`.

## Bundled web materials

The web build uses Source Sans 3 through Fontsource (**SIL OFL 1.1**), Hanzi Writer's
runtime (**MIT**), and Lucide icons (**ISC**, with some Feather-derived icons under
**MIT**). Their complete license texts ship in their npm packages. Other code dependencies
and their declared licenses are recorded by `backend/go.mod`, `backend/go.sum`,
`web/package.json`, and `web/bun.lock`.

## CC-CEDICT

Chinese-English dictionary entries. © CC-CEDICT contributors / MDBG.
Licensed **CC BY-SA 4.0** — https://cc-cedict.org/
Seeded into the dictionary tables in Phase 3; attribution also shown in-app (Sources line).
Vendored as `backend/data/downloads/cedict.txt.gz`, snapshot date
`2026-07-11T06:53:08Z`, SHA-256
`c3fc06d1f23bb1a4a1f7b7cabffb7f0c0a978aa521d2e775362bd0211676f191`.

## Hanzi Writer stroke data

Stroke order medians derived from `hanzi-writer-data` (chanind/hanzi-writer-data), which
derives from Make Me a Hanzi / Arphic Technology font outlines.
Licensed under the **Arphic Public License** — the full text is vendored as
`backend/data/strokes/ARPHICPL.TXT` alongside the data.
The source archive is `hanzi-writer-data` version 2.0.1, vendored as
`backend/data/downloads/hanzi-writer-data.tgz`, SHA-256
`72baf3d82b114e60d6e40ea05f24d2262a05cd39d544e2f322ba2fceb7beff15`.

## Make Me a Hanzi decompositions

Character component decompositions come from `dictionary.txt` in Make Me a Hanzi
(skishore/makemeahanzi), derived from Unihan and CJKlib. The data is licensed under the
**GNU Lesser General Public License, version 3 or later**. The full source notice and license
text are vendored at `backend/data/decompositions/LGPL-3.0.txt`.

The vendored dataset is pinned to commit
`bddc96d41bef78427ed0e034e9f7e31d71fd1b92` and has SHA-256
`744bb05d5b0742e9ee35c37791f94d56a173349b3367569e7ca11e510364d203`.

## OpenCC

Simplified ⇄ Traditional conversion dictionaries via `github.com/longbridge/opencc`.
Licensed **Apache-2.0**.

## Unihan

kMandarin/kHanyuPinyin readings used for the per-character pinyin fallback: the
authored fixtures derive from them, and the seed backfills long-tail readings for
codepoints CC-CEDICT lacks. © Unicode, Inc. —
[Unicode License v3](https://www.unicode.org/license.txt) (`Unicode-3.0`).
Vendored as `backend/data/downloads/unihan.zip`, pinned to the Unicode 17.0.0
release (https://www.unicode.org/Public/17.0.0/ucd/Unihan.zip), SHA-256
`f7a48b2b545acfaa77b2d607ae28747404ce02baefee16396c5d2d7a8ef34b5e`.

## Tatoeba

Example sentences (Mandarin with English translations) come from the Tatoeba corpus —
https://tatoeba.org/ — © Tatoeba contributors, licensed
[**CC BY 2.0 FR**](https://creativecommons.org/licenses/by/2.0/fr/deed.en).
Vendored as a compact derivative `backend/data/downloads/tatoeba_cmn_eng.tsv.gz`
(65,578 sentence pairs joined from the 2026-07-11 per-language exports;
SHA-256 `e64198609d7681970a92784bd318fd030ff26b4981901b3592e05db91c1da1dc`),
rebuildable with `fanti tatoeba-prepare`. Each row keeps its Tatoeba sentence id, so any
sentence can be traced to its authors at `https://tatoeba.org/en/sentences/show/<id>`.
Fanti joined the language exports, kept the lowest-id English translation per Mandarin
sentence, flattened field-separator whitespace, and compressed the result. Attribution also
appears in-app.

Character frequency ranks are another Fanti-created derivative of this corpus. At seed
time, Fanti counts Han characters in the Mandarin sentence field, orders them by descending
occurrence, and breaks ties by Unicode code point. English translations do not affect the
ranks. No separate character-frequency dataset is downloaded or redistributed.

## Project Gutenberg

Seeded classics (三國演義, 紅樓夢, 儒林外史) are fetched from Project Gutenberg.
Project Gutenberg identifies them as unrestricted by copyright in the United States;
operators outside the United States must verify local copyright status before downloading
or redistributing them.

## Wikimedia Commons ancient character forms

Historical-script SVGs come from the Wikimedia Commons Ancient Chinese Characters
project. The importer accepts only files whose Commons metadata identifies them as
**Public Domain** or **CC0**, and stores each file's exact Commons page URL with the
character-history record. Source links are also shown beside attested forms in the app.

The optional seed step checks the 500 most frequent course characters plus the authored
character set by default. Operators can raise the frequency-rank limit to extend coverage
incrementally without replacing existing records.
