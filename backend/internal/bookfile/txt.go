package bookfile

func parseTXT(data []byte) (Parsed, error) {
	text, err := decodeText(data)
	if err != nil {
		return Parsed{}, err
	}
	return Parsed{Chapters: splitChapters(text)}, nil
}
