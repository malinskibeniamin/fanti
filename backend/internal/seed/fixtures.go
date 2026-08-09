package seed

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/malinskibeniamin/fanti/backend/data"
)

// LocalizedText mirrors the fanti.v1 tri-lingual copy shape.
type LocalizedText struct {
	En string `json:"en"`
	Tc string `json:"tc"`
	Sc string `json:"sc"`
}

// FixtureCharacter is one authored character from characters.json.
type FixtureCharacter struct {
	Traditional        string        `json:"traditional"`
	Simplified         string        `json:"simplified"`
	Pinyin             string        `json:"pinyin"`
	Zhuyin             string        `json:"zhuyin"`
	Pos                string        `json:"pos"`
	Meaning            string        `json:"meaning"`
	MappingStatus      string        `json:"mappingStatus"`
	StrokeCount        int           `json:"strokeCount"`
	HskLevel           int           `json:"hskLevel"`
	FrequencyRank      int           `json:"frequencyRank"`
	Topics             []string      `json:"topics"`
	Story              string        `json:"story"`
	Mnemonic           string        `json:"mnemonic"`
	SimplificationNote string        `json:"simplificationNote"`
	RadicalParts       []RadicalPart `json:"radicalParts"`
	Examples           []Example     `json:"examples"`
	Siblings           []string      `json:"siblings"`
	StarterDeck        bool          `json:"starterDeck"`
}

// RadicalPart is one structural component.
type RadicalPart struct {
	Part string `json:"part"`
	Note string `json:"note"`
}

// Example is a levelled example sentence.
type Example struct {
	HskLevel int    `json:"hskLevel"`
	Chinese  string `json:"chinese"`
	English  string `json:"english"`
}

// Key returns the character's primary key: the first traditional form
// (the prototype writes the ambiguous 髮 entry as "髮 · 發").
func (c FixtureCharacter) Key() string {
	fields := strings.Fields(c.Traditional)
	if len(fields) == 0 {
		return c.Traditional
	}

	return fields[0]
}

// FixtureWord is one word card from words.json.
type FixtureWord struct {
	Word        string `json:"word"`
	Pinyin      string `json:"pinyin"`
	Pos         string `json:"pos"`
	Meaning     string `json:"meaning"`
	Simplified  string `json:"simplified"`
	Traditional string `json:"traditional"`
	Story       string `json:"story"`
}

// FixtureCompound is one compound from compounds.json.
type FixtureCompound struct {
	Word       string   `json:"word"`
	Pinyin     string   `json:"pinyin"`
	Characters []string `json:"characters"`
	Gloss      string   `json:"gloss"`
}

// FixtureMilestone is one milestone from milestones.json.
type FixtureMilestone struct {
	Threshold int    `json:"threshold"`
	En        string `json:"en"`
	Tc        string `json:"tc"`
	Sc        string `json:"sc"`
}

// FixtureStory is one graded story from graded-stories.json.
type FixtureStory struct {
	ID                    string        `json:"id"`
	LevelLabel            string        `json:"levelLabel"`
	CharCount             int           `json:"charCount"`
	Title                 string        `json:"title"`
	TitleSimplified       string        `json:"titleSimplified"`
	Blurb                 LocalizedText `json:"blurb"`
	TraditionalParagraphs []string      `json:"traditionalParagraphs"`
	SimplifiedParagraphs  []string      `json:"simplifiedParagraphs"`
}

// FixturePassage is the sample chapter from passage.json.
type FixturePassage struct {
	Title                 string   `json:"title"`
	TitleSimplified       string   `json:"titleSimplified"`
	TraditionalParagraphs []string `json:"traditionalParagraphs"`
	SimplifiedParagraphs  []string `json:"simplifiedParagraphs"`
}

// FixtureBook is one library book's metadata from seed-books.json.
type FixtureBook struct {
	ID             string          `json:"id"`
	Title          string          `json:"title"`
	TitleEnglish   string          `json:"titleEnglish"`
	Author         string          `json:"author"`
	CoverColor     string          `json:"coverColor"`
	Script         string          `json:"script"`
	SourceFormat   string          `json:"sourceFormat"`
	Description    string          `json:"description"`
	FileSizeLabel  string          `json:"fileSizeLabel"`
	MetadataFields []MetadataField `json:"metadataFields"`
}

// MetadataField is one label/value detail row.
type MetadataField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Fixtures bundles all embedded authored content.
type Fixtures struct {
	Characters []FixtureCharacter
	Words      []FixtureWord
	Compounds  []FixtureCompound
	Milestones []FixtureMilestone
	Stories    []FixtureStory
	Passage    FixturePassage
	Books      []FixtureBook
	CharPinyin map[string]string
}

// LoadFixtures decodes every embedded seed JSON.
func LoadFixtures() (Fixtures, error) {
	var f Fixtures

	for _, load := range []struct {
		file string
		into any
	}{
		{"characters.json", &f.Characters},
		{"words.json", &f.Words},
		{"compounds.json", &f.Compounds},
		{"milestones.json", &f.Milestones},
		{"graded-stories.json", &f.Stories},
		{"passage.json", &f.Passage},
		{"seed-books.json", &f.Books},
		{"char-pinyin.json", &f.CharPinyin},
	} {
		raw, err := data.SeedFS.ReadFile("seed/" + load.file)
		if err != nil {
			return Fixtures{}, fmt.Errorf("read %s: %w", load.file, err)
		}

		if err := json.Unmarshal(raw, load.into); err != nil {
			return Fixtures{}, fmt.Errorf("decode %s: %w", load.file, err)
		}
	}

	return f, nil
}
