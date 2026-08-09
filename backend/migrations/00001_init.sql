-- +goose Up

-- Dictionary -----------------------------------------------------------------

CREATE TABLE dict_entries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    traditional TEXT NOT NULL,
    simplified TEXT NOT NULL,
    pinyin TEXT NOT NULL,
    definitions TEXT[] NOT NULL DEFAULT '{}'
);

CREATE INDEX dict_entries_traditional_idx ON dict_entries (traditional);
CREATE INDEX dict_entries_simplified_idx ON dict_entries (simplified);
CREATE INDEX dict_entries_pinyin_idx ON dict_entries (pinyin);

CREATE TABLE characters (
    traditional TEXT PRIMARY KEY,
    simplified TEXT NOT NULL,
    pinyin TEXT NOT NULL DEFAULT '',
    zhuyin TEXT NOT NULL DEFAULT '',
    pos TEXT NOT NULL DEFAULT '',
    meaning TEXT NOT NULL DEFAULT '',
    mapping_status TEXT NOT NULL DEFAULT 'exact',
    stroke_count INT NOT NULL DEFAULT 0,
    hsk_level INT NOT NULL DEFAULT 0,
    frequency_rank INT NOT NULL DEFAULT 0,
    topics TEXT[] NOT NULL DEFAULT '{}',
    story TEXT NOT NULL DEFAULT '',
    mnemonic TEXT NOT NULL DEFAULT '',
    simplification_note TEXT NOT NULL DEFAULT '',
    radical_parts JSONB NOT NULL DEFAULT '[]',
    examples JSONB NOT NULL DEFAULT '[]',
    siblings TEXT[] NOT NULL DEFAULT '{}',
    starter_deck BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX characters_simplified_idx ON characters (simplified);
CREATE INDEX characters_hsk_idx ON characters (hsk_level);

-- Per-character pinyin fallback for reader ruby text.
CREATE TABLE char_pinyin (
    ch TEXT PRIMARY KEY,
    pinyin TEXT NOT NULL
);

-- Hanzi Writer stroke medians (Arphic Public License — see NOTICES.md).
CREATE TABLE stroke_data (
    ch TEXT PRIMARY KEY,
    medians JSONB NOT NULL,
    stroke_count INT NOT NULL
);

CREATE TABLE compounds (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    word TEXT NOT NULL UNIQUE,
    pinyin TEXT NOT NULL DEFAULT '',
    chars TEXT[] NOT NULL DEFAULT '{}',
    gloss TEXT NOT NULL DEFAULT ''
);

-- Word cards for the flashcard word mode.
CREATE TABLE word_cards (
    word TEXT PRIMARY KEY,
    pinyin TEXT NOT NULL DEFAULT '',
    pos TEXT NOT NULL DEFAULT '',
    meaning TEXT NOT NULL DEFAULT '',
    simplified TEXT NOT NULL DEFAULT '',
    traditional TEXT NOT NULL DEFAULT '',
    story TEXT NOT NULL DEFAULT ''
);

-- Library --------------------------------------------------------------------

CREATE TABLE books (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    title_english TEXT NOT NULL DEFAULT '',
    author TEXT NOT NULL DEFAULT '',
    script TEXT NOT NULL DEFAULT 'traditional',
    source_format TEXT NOT NULL DEFAULT 'epub',
    cover_color TEXT NOT NULL DEFAULT '#8f1d18',
    reading_progress DOUBLE PRECISION NOT NULL DEFAULT 0,
    current_chapter_index INT NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT '',
    file_size_label TEXT NOT NULL DEFAULT '',
    metadata_fields JSONB NOT NULL DEFAULT '[]',
    graded_story BOOLEAN NOT NULL DEFAULT FALSE,
    level_label TEXT NOT NULL DEFAULT '',
    char_count BIGINT NOT NULL DEFAULT 0,
    epub_path TEXT NOT NULL DEFAULT '',
    create_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chapters (
    book_id TEXT NOT NULL REFERENCES books (id) ON DELETE CASCADE,
    idx INT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    -- Paragraphs as ordered text arrays, one per script.
    traditional_paragraphs TEXT[] NOT NULL DEFAULT '{}',
    simplified_paragraphs TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (book_id, idx)
);

-- Conversions ----------------------------------------------------------------

CREATE TABLE conversions (
    id TEXT PRIMARY KEY,
    state TEXT NOT NULL DEFAULT 'ready',
    filename TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT '',
    detected_script TEXT NOT NULL DEFAULT '',
    char_count BIGINT NOT NULL DEFAULT 0,
    unit_counts JSONB NOT NULL DEFAULT '[]',
    settings JSONB NOT NULL DEFAULT '{}',
    layout JSONB NOT NULL DEFAULT '{}',
    progress_percent INT NOT NULL DEFAULT 0,
    report JSONB,
    error_message TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    -- Parsed source chapters: [{title, paragraphs: []}].
    source JSONB,
    -- Converted chapters: [{title, paragraphs: []}].
    result JSONB,
    create_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    update_time TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Study ----------------------------------------------------------------------

CREATE TABLE reviews (
    ch TEXT PRIMARY KEY,
    srs_level INT NOT NULL DEFAULT 0,
    due_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    mistake_count INT NOT NULL DEFAULT 0,
    learned BOOLEAN NOT NULL DEFAULT FALSE,
    in_deck BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE practice_days (
    day DATE PRIMARY KEY
);

CREATE TABLE learning_records (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    record_type TEXT NOT NULL,
    ch TEXT NOT NULL DEFAULT '',
    milestone_threshold INT NOT NULL DEFAULT 0,
    record_time TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cloze_cards (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ch TEXT NOT NULL,
    sentence TEXT NOT NULL,
    UNIQUE (ch, sentence)
);

-- Single-row learner profile.
CREATE TABLE study_profile (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    goal TEXT NOT NULL DEFAULT 'practical',
    mission TEXT NOT NULL DEFAULT ''
);

INSERT INTO study_profile (id) VALUES (TRUE);

CREATE TABLE quizzes (
    id TEXT PRIMARY KEY,
    -- Questions with server-side answers: [{type, prompt, character, ttsText, options, answer}].
    questions JSONB NOT NULL DEFAULT '[]',
    current_index INT NOT NULL DEFAULT 0,
    score INT NOT NULL DEFAULT 0,
    finished BOOLEAN NOT NULL DEFAULT FALSE,
    mistakes TEXT[] NOT NULL DEFAULT '{}',
    lesson_ch TEXT NOT NULL DEFAULT '',
    create_time TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE milestones (
    threshold INT PRIMARY KEY,
    label_en TEXT NOT NULL,
    label_tc TEXT NOT NULL,
    label_sc TEXT NOT NULL
);

-- +goose Down

DROP TABLE milestones;
DROP TABLE quizzes;
DROP TABLE study_profile;
DROP TABLE cloze_cards;
DROP TABLE learning_records;
DROP TABLE practice_days;
DROP TABLE reviews;
DROP TABLE conversions;
DROP TABLE chapters;
DROP TABLE books;
DROP TABLE word_cards;
DROP TABLE compounds;
DROP TABLE stroke_data;
DROP TABLE char_pinyin;
DROP TABLE characters;
DROP TABLE dict_entries;
