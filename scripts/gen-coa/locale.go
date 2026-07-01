package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// localeFile is the on-disk shape of locales/<lang>.json. _meta is preserved
// verbatim (RawMessage) so re-emitting never churns it; coa is the code→display
// map the coverage test (IT14) asserts against.
type localeFile struct {
	Meta json.RawMessage   `json:"_meta"`
	Coa  map[string]string `json:"coa"`
}

// syncLocale ensures every account code has a key in the locale file. Missing
// keys are stubbed with the canonical English name (the translator overwrites
// later); a locale is only rewritten when something was added, so a fully
// translated file stays byte-stable across runs. Returns the number added.
func syncLocale(path string, codes []int, names map[int]string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read locale %s: %w", path, err)
	}
	var lf localeFile
	if err := json.Unmarshal(b, &lf); err != nil {
		return 0, fmt.Errorf("parse locale %s: %w", path, err)
	}
	if lf.Coa == nil {
		lf.Coa = map[string]string{}
	}

	added := 0
	for _, code := range codes {
		key := fmt.Sprintf("%d", code)
		if _, ok := lf.Coa[key]; !ok {
			lf.Coa[key] = names[code]
			added++
		}
	}
	if added == 0 {
		return 0, nil
	}

	out, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("marshal locale %s: %w", path, err)
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return 0, fmt.Errorf("write locale %s: %w", path, err)
	}
	return added, nil
}
